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

package atc_status

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
)

const (
	// maxATCStatusResponseBytes bounds the memory used to decode an ATC response.
	maxATCStatusResponseBytes = 64 * 1024
)

// ATCStatus represents a feeder status response from the ATC API.
type ATCStatus struct {
	// Status contains the connection status for each supported protocol.
	Status StatusEntry `json:"status"`
}

// StatusEntry contains the ADS-B and MLAT connection statuses.
type StatusEntry struct {
	// ADSB is the ADS-B feed connection status.
	ADSB ProtocolStatus `json:"adsb"`
	// MLAT is the MLAT feed connection status.
	MLAT ProtocolStatus `json:"mlat"`
}

// ProtocolStatus describes the connection status of a feeder protocol.
type ProtocolStatus struct {
	// status is the human-readable health derived from Connected.
	status string
	// Connected reports whether the feed is currently connected.
	Connected bool `json:"connected"`
	// LastSeen records when the feed was last observed.
	LastSeen time.Time `json:"last_seen"`
}

var (
	// ctx is the active feeder status check context.
	ctx context.Context

	// cancelFunc cancels the active feeder status check.
	cancelFunc context.CancelFunc

	// randSeconds is the random component used to jitter the polling interval.
	randSeconds = rand.Intn(120)

	// ErrResponseNotOK indicates that ATC returned a non-OK HTTP response.
	ErrResponseNotOK = errors.New("HTTP response code not OK")

	mu sync.RWMutex
)

// getStatusFromATC retrieves the current feeder status from the ATC API.
func (S *ATCStatus) getStatusFromATC(atcUrl, apiKey string) error {

	// Build the ATC API request URL.
	requestURL, err := url.JoinPath(atcUrl, "api", "v1", "feeders", apiKey, "status.json")
	if err != nil {
		log.Err(err).Str("url", requestURL).Str("atcurl", atcUrl).Msg("could not form request URL")
		return err
	}
	res, err := http.Get(requestURL)
	if err != nil {
		log.Err(err).Str("url", requestURL).Msg("error making feeder status http request")
		return err
	}

	// Ensure the response body is closed.
	defer func() {
		_ = res.Body.Close()
	}()

	// Check the HTTP response status.
	if res.StatusCode != http.StatusOK {
		log.Err(err).Str("url", requestURL).Msg("bad response from feeder status http request")
		return ErrResponseNotOK
	}

	// Stream the JSON response into a temporary value.
	// Limiting the reader avoids retaining both an unbounded response body and its decoded representation.
	var nextStatus ATCStatus
	decoder := json.NewDecoder(io.LimitReader(res.Body, maxATCStatusResponseBytes))
	err = decoder.Decode(&nextStatus)
	if err != nil {
		log.Err(err).Msg("error decoding json from feeder status http response body")
		return err
	}

	// Set a human-readable status for each protocol.
	if nextStatus.Status.ADSB.Connected {
		nextStatus.Status.ADSB.status = "healthy"
	} else {
		nextStatus.Status.ADSB.status = "unhealthy"
	}
	if nextStatus.Status.MLAT.Connected {
		nextStatus.Status.MLAT.status = "healthy"
	} else {
		nextStatus.Status.MLAT.status = "unhealthy"
	}

	// Only update the status after decoding succeeds.
	mu.Lock()
	*S = nextStatus
	mu.Unlock()

	return nil
}

// Start periodically retrieves feeder status from ATC and writes it to the
// application log. The interval is jittered by up to one minute.
func Start(
	parentContext context.Context,
	atcUrl, apiKey string,
	interval int,
	reg prometheus.Registerer,
) {
	ctx, cancelFunc = context.WithCancel(parentContext)
	S := ATCStatus{}

	if reg != nil {
		colADSBHealth := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace:   "plane.watch",
			Subsystem:   "ATC Status",
			Name:        "ADSB Healthy",
			Help:        "Beast feeding status from plane watch ATC: 1 if BEAST feeding is successful, 0 if not",
			Unit:        "",
			ConstLabels: nil,
		}, func() float64 {
			mu.RLock()
			defer mu.RUnlock()
			if S.Status.ADSB.Connected {
				return 1
			} else {
				return 0
			}
		})
		if err := reg.Register(colADSBHealth); err != nil {
			log.Err(err).Msg("could not register Prometheus gauge for ADSB health check")
		} else {
			defer reg.Unregister(colADSBHealth)
		}
		colMLATHealth := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace:   "plane.watch",
			Subsystem:   "ATC Status",
			Name:        "MLAT Healthy",
			Help:        "MLAT feeding status from plane watch ATC: 1 if MLAT feeding is successful, 0 if not",
			Unit:        "",
			ConstLabels: nil,
		}, func() float64 {
			mu.RLock()
			defer mu.RUnlock()
			if S.Status.MLAT.Connected {
				return 1
			} else {
				return 0
			}
		})
		if err := reg.Register(colMLATHealth); err != nil {
			log.Err(err).Msg("could not register Prometheus gauge for MLAT health check")
		} else {
			defer reg.Unregister(colMLATHealth)
		}
	}

	for {
		select {

		// Stop when the context is cancelled.
		case <-ctx.Done():
			log.Debug().Msg("stopped")
			return

		// Retrieve the status after the jittered interval.
		case <-time.After(time.Duration((interval - 60 + randSeconds)) * time.Second):

			// Get the current status from ATC.
			err := S.getStatusFromATC(atcUrl, apiKey)

			// Log the status when the request succeeds.
			if err == nil {

				if S.Status.ADSB.Connected && S.Status.MLAT.Connected {

					// Log healthy connections at info level.
					log.Info().Str("ADSB", S.Status.ADSB.status).Str("MLAT", S.Status.MLAT.status).Msg("atc.plane.watch reported connection status")

				} else {
					// Log unhealthy connections at warning level.
					log.Warn().Str("ADSB", S.Status.ADSB.status).Str("MLAT", S.Status.MLAT.status).Msg("atc.plane.watch reported connection status")
				}
			}
		}
	}
}

// Stop stops the ATC feeder status check.
func Stop() {
	log.Debug().Msg("stopping")
	cancelFunc()
}
