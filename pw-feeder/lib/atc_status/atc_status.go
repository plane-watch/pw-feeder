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

	// Read the response body.
	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Err(err).Msg("error reading feeder status http response body")
		return err
	}

	// Decode the JSON response.
	mu.Lock()
	defer mu.Unlock()
	err = json.Unmarshal(body, &S)
	if err != nil {
		log.Err(err).Msg("error unmarshalling json from feeder status http response body")
		return err
	}

	// Set a human-readable status for each protocol.
	if S.Status.ADSB.Connected {
		S.Status.ADSB.status = "healthy"
	} else {
		S.Status.ADSB.status = "unhealthy"
	}
	if S.Status.MLAT.Connected {
		S.Status.MLAT.status = "healthy"
	} else {
		S.Status.MLAT.status = "unhealthy"
	}

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
			Help:        "1 if BEAST feeding is successful, 0 if not.",
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
			Help:        "1 if MLAT feeding is successful, 0 if not.",
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
