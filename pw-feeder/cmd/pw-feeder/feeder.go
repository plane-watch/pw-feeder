// Copyright (C) 2024 Plane Watch
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This file is part of pw-feeder.
//
// pw-feeder is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// pw-feeder is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with pw-feeder. If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/signal"
	"pw-feeder/lib/atc_status"
	"pw-feeder/lib/connproxy"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
)

const (
	atcStatusIntervalSeconds = 300
	metricsShutdownTimeout   = 5 * time.Second
)

// runFeeder prepares, starts, and gracefully stops the feeder services.
func runFeeder(parentCtx context.Context, command *cli.Command) error {
	cfg := configFromCommand(command)
	redactList[cfg.apiKey] = "[API_KEY_REDACTED]"
	logStartup(cfg)

	// set up cancel function to stop goroutines
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	// set up new context that will allow stopping on SIGTERM
	runCtx, stopSignals := signal.NotifyContext(ctx, syscall.SIGTERM)
	defer stopSignals()

	metrics, err := prepareMetrics(runCtx, cfg)
	if err != nil {
		return err
	}

	mlatListener, err := prepareMLATListener(cfg)
	if err != nil {
		return err
	}
	if mlatListener != nil {
		defer func() {
			_ = mlatListener.Close()
		}()
	}

	err = metrics.Start()
	if err != nil {
		return err
	}

	workers := startFeederServices(runCtx, cfg, mlatListener, metrics.Registerer())
	runErr := waitForShutdown(runCtx, metrics.Errors())

	// Stop the feeder services before shutting down their metrics endpoint.
	cancel()
	shutdownErr := shutdownMetrics(metrics)
	workers.Wait()

	return errors.Join(runErr, shutdownErr)
}

// logStartup reports the feeder build and version information.
func logStartup(cfg feederConfig) {
	log.Info().
		Str("commithash", commithash()[:7]).
		Str("version", cfg.version).
		Msg("plane.watch feeder started")
}

// prepareMLATListener creates the local listener used by mlat-client.
func prepareMLATListener(cfg feederConfig) (net.Listener, error) {
	if !cfg.mlatEnabled {
		return nil, nil
	}
	return net.Listen("tcp", cfg.mlatListen)
}

// startFeederServices starts the BEAST proxy, optional MLAT proxy, and ATC
// status updater.
func startFeederServices(
	ctx context.Context,
	cfg feederConfig,
	mlatListener net.Listener,
	reg prometheus.Registerer,
) *sync.WaitGroup {
	workers := &sync.WaitGroup{}

	workers.Go(func() {
		connproxy.ProxyBEASTConnection(
			ctx,
			"BEAST",
			cfg.beastSource,
			cfg.beastEndpoint,
			cfg.apiKey,
			cfg.insecure,
			reg,
		)
	})

	if cfg.mlatEnabled {
		workers.Go(func() {
			connproxy.ProxyMLATConnection(
				ctx,
				"MLAT",
				mlatListener,
				cfg.mlatEndpoint,
				cfg.apiKey,
				cfg.insecure,
				reg,
			)
		})
	}

	workers.Go(func() {
		atc_status.Start(
			ctx,
			cfg.atcURL,
			cfg.apiKey,
			atcStatusIntervalSeconds,
			reg,
		)
	})

	return workers
}

// waitForShutdown waits for a shutdown signal, parent cancellation, or metrics
// server failure.
func waitForShutdown(ctx context.Context, metricsErrors <-chan error) error {
	select {
	case <-ctx.Done():
		log.Info().Msg("shutdown requested, stopping")
		return nil
	case err := <-metricsErrors:
		return fmt.Errorf("metrics listener failed: %w", err)
	}
}

// shutdownMetrics gives the metrics server a bounded graceful-shutdown window.
func shutdownMetrics(metrics *metricsService) error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), metricsShutdownTimeout)
	defer cancel()
	return metrics.Shutdown(shutdownCtx)
}
