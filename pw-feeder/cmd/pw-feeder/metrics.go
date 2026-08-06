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
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
)

// metricsService owns the Prometheus registry and HTTP server lifecycle.
type metricsService struct {
	registry *prometheus.Registry
	server   *http.Server
	errCh    chan error
}

// prepareMetrics creates the metrics registry and HTTP server without starting
// the listener. A disabled service is represented by a no-op metricsService.
func prepareMetrics(ctx context.Context, cfg feederConfig) (*metricsService, error) {
	service := &metricsService{}
	if !cfg.metricsEnabled {
		return service, nil
	}

	registry := prometheus.NewRegistry()
	if cfg.debug {
		if err := registry.Register(collectors.NewGoCollector()); err != nil {
			return nil, fmt.Errorf("could not register Go metrics collector: %w", err)
		}
		if err := registry.Register(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{})); err != nil {
			return nil, fmt.Errorf("could not register process metrics collector: %w", err)
		}
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	service.registry = registry
	service.errCh = make(chan error, 1)
	service.server = &http.Server{
		Addr:              cfg.metricsAddress,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}

	return service, nil
}

// Start binds and starts the metrics HTTP server. Binding synchronously ensures
// address errors are reported before the feeder services start.
func (service *metricsService) Start() error {
	if service == nil || service.server == nil {
		return nil
	}

	listener, err := net.Listen("tcp", service.server.Addr)
	if err != nil {
		return fmt.Errorf("could not start metrics listener: %w", err)
	}

	go func() {
		err := service.server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			service.errCh <- err
		}
	}()

	log.Info().Msgf("metrics available at http://%s/metrics", listener.Addr())
	return nil
}

// Registerer returns the registry used by feeder services, or nil when metrics
// are disabled.
func (service *metricsService) Registerer() prometheus.Registerer {
	if service == nil {
		return nil
	}
	return service.registry
}

// Errors returns asynchronous metrics server failures. A nil channel disables
// the corresponding select case when metrics are disabled.
func (service *metricsService) Errors() <-chan error {
	if service == nil {
		return nil
	}
	return service.errCh
}

// Shutdown gracefully stops the metrics HTTP server.
func (service *metricsService) Shutdown(ctx context.Context) error {
	if service == nil || service.server == nil {
		return nil
	}
	if err := service.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("could not stop metrics listener: %w", err)
	}
	return nil
}
