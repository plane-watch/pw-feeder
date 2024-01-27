package atc_status

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

type ATCStatus struct {
	Status StatusEntry `json:"status"`
}

type StatusEntry struct {
	ADSB ProtocolStatus `json:"adsb"`
	MLAT ProtocolStatus `json:"mlat"`
}

type ProtocolStatus struct {
	status    string
	Connected bool      `json:"connected"`
	LastSeen  time.Time `json:"last_seen"`
}

var (
	ctx         context.Context
	cancelFunc  context.CancelFunc
	randSeconds = rand.Intn(120)

	ErrResponseNotOK = errors.New("HTTP response code not OK")
)

func (S *ATCStatus) getStatusFromATC(atcUrl, apiKey string) error {

	// make atc api request
	requestURL := fmt.Sprintf("%s/api/v1/feeders/%s/status.json", atcUrl, apiKey)
	res, err := http.Get(requestURL)
	if err != nil {
		log.Err(err).Str("url", requestURL).Msg("error making http request")
		return err
	}
	if res.StatusCode != http.StatusOK {
		log.Err(err).Str("url", requestURL).Msg("bad response")
		return ErrResponseNotOK
	}

	// defer closure response body
	defer res.Body.Close()

	// read response body
	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Err(err).Msg("error reading http response body")
		return err
	}

	// unmarshall response
	err = json.Unmarshal(body, &S)
	if err != nil {
		log.Err(err).Msg("error unmarshalling json")
		return err
	}

	// set status accordingly
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

// Start starts the process to check feeder status from ATC every interval seconds (+/- random seconds up to 1 minute)
// Feeder status from ATC will be logged to the application log.
func Start(parentContext context.Context, atcUrl, apiKey string, interval int) {
	ctx, cancelFunc = context.WithCancel(parentContext)
	S := ATCStatus{}
	for {
		select {

		// stop on context closure
		case <-ctx.Done():
			log.Debug().Msg("stopped")
			return

		// otherwise keep on running
		case <-time.After(time.Duration((interval - 60 + randSeconds)) * time.Second):

			// get status from ATC
			err := S.getStatusFromATC(atcUrl, apiKey)

			// if no error, then log status
			if err == nil {

				if S.Status.ADSB.Connected && S.Status.MLAT.Connected {

					// if connected ok, log with info level
					log.Info().Str("ADSB", S.Status.ADSB.status).Str("MLAT", S.Status.MLAT.status).Msg("atc.plane.watch reported connection status")

				} else {
					// if not connected ok, log with warning level
					log.Warn().Str("ADSB", S.Status.ADSB.status).Str("MLAT", S.Status.MLAT.status).Msg("atc.plane.watch reported connection status")
				}
			}
		}
	}
}

// Stop stops the process to check feeder status from ATC
func Stop() {
	log.Debug().Msg("stopping")
	cancelFunc()
}
