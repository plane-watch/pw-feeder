package main

import (
	"encoding/json"
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
	Connected bool      `json:"connected"`
	LastSeen  time.Time `json:"last_seen"`
}

func (S *ATCStatus) getStatusFromATC(atcUrl, apiKey string) error {

	// make atc api request
	requestURL := fmt.Sprintf("%s/api/v1/feeders/%s/status.json", atcUrl, apiKey)
	res, err := http.Get(requestURL)
	if err != nil {
		log.Err(err).Str("url", requestURL).Msg("error making http request")
		return err
	}

	// read response body
	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Err(err).Msg("error reading http response body")
		return err
	}

	// close response body
	defer res.Body.Close()

	// unmarshall response
	err = json.Unmarshal(body, &S)
	if err != nil {
		log.Err(err).Msg("error unmarshalling json")
		return err
	}

	return nil
}

func initStatusUpdater(atcUrl, apiKey string, whenDone func()) {
	S := ATCStatus{}
	for {
		time.Sleep(time.Duration((240 + rand.Intn(120))) * time.Second) // 5 mins +/- up to 1 min
		err := S.getStatusFromATC(atcUrl, apiKey)
		if err == nil {
			log.Info().Bool("ADSB", S.Status.ADSB.Connected).Bool("MLAT", S.Status.MLAT.Connected).Msg("atc.plane.watch connection status")
		}
	}
	whenDone()
}
