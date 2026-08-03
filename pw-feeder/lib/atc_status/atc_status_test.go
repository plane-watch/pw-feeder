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
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil/promlint"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// MockServerTestScenarioWorking selects healthy ADS-B and MLAT responses.
	MockServerTestScenarioWorking = iota
	// MockServerTestScenarioBEASTOnly selects a healthy ADS-B-only response.
	MockServerTestScenarioBEASTOnly
	// MockServerTestScenarioMLATOnly selects a healthy MLAT-only response.
	MockServerTestScenarioMLATOnly
	// MockServerTestScenarioNoResponse selects an unavailable mock server.
	MockServerTestScenarioNoResponse
	// MockServerTestScenarioBadRequest selects an HTTP bad-request response.
	MockServerTestScenarioBadRequest
	// MockServerTestScenarioInvalidJSON selects a malformed JSON response.
	MockServerTestScenarioInvalidJSON
	// MockServerTestScenarioServerError selects an empty response body.
	MockServerTestScenarioServerError
	// MockServerTestScenarioOversizedResponse selects a response over the decode limit.
	MockServerTestScenarioOversizedResponse
)

var (
	// TestFeederAPIKey identifies the feeder used in mock ATC requests.
	TestFeederAPIKey = uuid.New()
)

// init configures console logging for the package tests.
func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.UnixDate})
}

// PrepMockATCServer starts an ATC test server for the selected scenario.
func PrepMockATCServer(t *testing.T, testScenario int) *httptest.Server {

	// Based on: https://medium.com/zus-health/mocking-outbound-http-requests-in-go-youre-probably-doing-it-wrong-60373a38d2aa

	// Prepare the test server.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		switch r.URL.Path {

		case fmt.Sprintf("/api/v1/feeders/%s/status.json", strings.ToLower(TestFeederAPIKey.String())):

			// Check the request.
			assert.Equal(t, http.MethodGet, r.Method)

			// Prepare the mock response.
			resp := `{
				"status": {
				  "adsb": {
					"connected": %t,
					"last_seen": "2023-12-29T09:02:34.515909245Z"
				  },
				  "mlat": {
					"connected": %t,
					"last_seen": "2023-12-29T09:02:05.791969689Z"
				  }
				}
			  }`
			switch testScenario {
			case MockServerTestScenarioWorking:
				resp = fmt.Sprintf(resp, true, true)
			case MockServerTestScenarioBEASTOnly:
				resp = fmt.Sprintf(resp, true, false)
			case MockServerTestScenarioMLATOnly:
				resp = fmt.Sprintf(resp, false, true)
			default:
				resp = fmt.Sprintf(resp, false, false)
			}

			// Write the response status.
			switch testScenario {
			case MockServerTestScenarioBadRequest:
				w.WriteHeader(http.StatusBadRequest)
			default:
				w.WriteHeader(http.StatusOK)
			}

			// Write the response body.
			switch testScenario {
			case MockServerTestScenarioInvalidJSON:
				_, _ = w.Write([]byte(resp)[2:])
			case MockServerTestScenarioServerError:
				return
			case MockServerTestScenarioOversizedResponse:
				_, _ = w.Write([]byte(`{"padding":"` + strings.Repeat("x", maxATCStatusResponseBytes) + `"}`))
			default:
				_, _ = w.Write([]byte(resp))
			}

		default:
			t.Log("invalid request URL:", r.URL.Path)
			t.FailNow()
		}

	}))

	if testScenario == MockServerTestScenarioNoResponse {
		server.Close()
	}

	return server
}

// TestGetStatusFromATC verifies status retrieval and error handling.
func TestGetStatusFromATC(t *testing.T) {

	t.Run("connection refused", func(t *testing.T) {
		// Start the test server.
		testServer := PrepMockATCServer(t, MockServerTestScenarioBadRequest)
		testServer.Close()

		// Retrieve the feeder status.
		S := ATCStatus{}
		err := S.getStatusFromATC(testServer.URL, TestFeederAPIKey.String())

		// Check the result.
		require.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "connection refused"))

	})

	t.Run("bad request", func(t *testing.T) {
		// Start the test server.
		testServer := PrepMockATCServer(t, MockServerTestScenarioBadRequest)
		t.Cleanup(func() {
			testServer.Close()
		})

		// Retrieve the feeder status.
		S := ATCStatus{}
		err := S.getStatusFromATC(testServer.URL, TestFeederAPIKey.String())

		// Check the result.
		require.Error(t, err)
		assert.Equal(t, ErrResponseNotOK.Error(), err.Error())
	})

	t.Run("server error", func(t *testing.T) {
		// Start the test server.
		testServer := PrepMockATCServer(t, MockServerTestScenarioServerError)
		t.Cleanup(func() {
			testServer.Close()
		})

		// Retrieve the feeder status.
		S := ATCStatus{}
		err := S.getStatusFromATC(testServer.URL, TestFeederAPIKey.String())

		// Check the result.
		require.Error(t, err)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		// Start the test server.
		testServer := PrepMockATCServer(t, MockServerTestScenarioInvalidJSON)
		t.Cleanup(func() {
			testServer.Close()
		})

		// Retrieve the feeder status.
		S := ATCStatus{}
		err := S.getStatusFromATC(testServer.URL, TestFeederAPIKey.String())

		// Check the result.
		require.Error(t, err)
		assert.ErrorContains(t, err, "json")
	})

	t.Run("oversized response", func(t *testing.T) {
		// Start the test server.
		testServer := PrepMockATCServer(t, MockServerTestScenarioOversizedResponse)
		t.Cleanup(func() {
			testServer.Close()
		})

		// Retrieve the feeder status.
		S := ATCStatus{}
		err := S.getStatusFromATC(testServer.URL, TestFeederAPIKey.String())

		// The response is truncated at the configured limit and cannot be decoded.
		require.Error(t, err)
	})

	t.Run("working BEAST", func(t *testing.T) {
		// Start the test server.
		testServer := PrepMockATCServer(t, MockServerTestScenarioBEASTOnly)
		t.Cleanup(func() {
			testServer.Close()
		})

		// Retrieve the feeder status.
		S := ATCStatus{}
		err := S.getStatusFromATC(testServer.URL, TestFeederAPIKey.String())

		// Check the result.
		require.NoError(t, err)
		assert.Equal(t, "healthy", S.Status.ADSB.status)
		assert.Equal(t, true, S.Status.ADSB.Connected)
	})

	t.Run("working MLAT", func(t *testing.T) {
		// Start the test server.
		testServer := PrepMockATCServer(t, MockServerTestScenarioMLATOnly)
		t.Cleanup(func() {
			testServer.Close()
		})

		// Retrieve the feeder status.
		S := ATCStatus{}
		err := S.getStatusFromATC(testServer.URL, TestFeederAPIKey.String())

		// Check the result.
		require.NoError(t, err)
		assert.Equal(t, "healthy", S.Status.MLAT.status)
		assert.Equal(t, true, S.Status.MLAT.Connected)
	})

	t.Run("working BEAST and MLAT", func(t *testing.T) {
		// Start the test server.
		testServer := PrepMockATCServer(t, MockServerTestScenarioWorking)
		t.Cleanup(func() {
			testServer.Close()
		})

		// Retrieve the feeder status.
		S := ATCStatus{}
		err := S.getStatusFromATC(testServer.URL, TestFeederAPIKey.String())

		// Check the result.
		require.NoError(t, err)
		assert.Equal(t, "healthy", S.Status.ADSB.status)
		assert.Equal(t, "healthy", S.Status.MLAT.status)
		assert.Equal(t, true, S.Status.ADSB.Connected)
		assert.Equal(t, true, S.Status.MLAT.Connected)
	})
}

// TestStartStop verifies that the status loop starts and stops cleanly.
func TestStartStop(t *testing.T) {
	t.Run("working BEAST only", func(t *testing.T) {
		// Start the test server.
		testServer := PrepMockATCServer(t, MockServerTestScenarioBEASTOnly)
		t.Cleanup(func() {
			testServer.Close()
		})

		// Reduce the random wait.
		randSeconds = 1

		// Prepare the parent context and wait group.
		testCtx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()
		wg := sync.WaitGroup{}

		// Start the status loop.
		wg.Go(func() {
			Start(testCtx, testServer.URL, TestFeederAPIKey.String(), 60, nil)
		})

		// Wait for status logging.
		time.Sleep(time.Second * 10)

		// Stop the status loop.
		wg.Go(func() {
			Stop()
		})

		// Wait for all goroutines to finish.
		wg.Wait()

		// fmt.Println(testCtx.Deadline())
		// fmt.Println(testCtx.Err())

	})

	t.Run("working BEAST & MLAT", func(t *testing.T) {
		// Start the test server.
		testServer := PrepMockATCServer(t, MockServerTestScenarioWorking)
		t.Cleanup(func() {
			testServer.Close()
		})

		// Reduce the random wait.
		randSeconds = 1

		// Prepare the parent context and wait group.
		testCtx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()
		wg := sync.WaitGroup{}

		// Start the status loop.
		wg.Go(func() {
			Start(testCtx, testServer.URL, TestFeederAPIKey.String(), 60, nil)
		})

		// Wait for status logging.
		time.Sleep(time.Second * 10)

		// Stop the status loop.
		wg.Go(func() {
			Stop()
		})

		// Wait for all goroutines to finish.
		wg.Wait()

		// fmt.Println(testCtx.Deadline())
		// fmt.Println(testCtx.Err())

	})
}

// TestStartUnregistersMetrics verifies that status collectors are removed from
// the supplied registry when the status loop stops.
func TestStartUnregistersMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	testCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)
		Start(testCtx, "http://unused", TestFeederAPIKey.String(), 3600, reg)
	}()

	require.Eventually(t, func() bool {
		metricFamilies, err := reg.Gather()
		return err == nil && len(metricFamilies) == 1 && len(metricFamilies[0].GetMetric()) == 2
	}, time.Second, 10*time.Millisecond)

	metricFamilies, err := reg.Gather()
	require.NoError(t, err)
	require.Len(t, metricFamilies, 1)
	metricFamily := metricFamilies[0]
	assert.Equal(t, "pwfeeder_atc_feed_healthy", metricFamily.GetName())
	assert.Equal(t, "GAUGE", metricFamily.GetType().String())
	assert.Empty(t, metricFamily.GetUnit())

	protocolValues := make(map[string]float64, 2)
	for _, metric := range metricFamily.GetMetric() {
		require.Len(t, metric.GetLabel(), 1)
		label := metric.GetLabel()[0]
		assert.Equal(t, "protocol", label.GetName())
		protocolValues[label.GetValue()] = metric.GetGauge().GetValue()
	}
	assert.Equal(t, map[string]float64{"adsb": 0, "mlat": 0}, protocolValues)

	problems, err := promlint.NewWithMetricFamilies(metricFamilies).Lint()
	require.NoError(t, err)
	assert.Empty(t, problems)

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("status loop did not stop after context cancellation")
	}

	metricFamilies, err = reg.Gather()
	require.NoError(t, err)
	assert.Empty(t, metricFamilies)
}
