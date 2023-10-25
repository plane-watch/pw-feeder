package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
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

func (S *ATCStatus) getStatusFromATC(atcUrl, apiKey string) {

	requestURL := fmt.Sprintf("%s/api/v1/feeders/%s/status.json", atcUrl, apiKey)
	res, err := http.Get(requestURL)
	if err != nil {
		fmt.Printf("error making http request: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("client: got response!\n")
	fmt.Printf("client: status code: %d\n", res.StatusCode)

	// read response body
	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// close response body
	res.Body.Close()

	// print response body
	fmt.Println(string(body))

	err = json.Unmarshal(body, &S)

	fmt.Println(S)

}

func initStatusUpdater(atcUrl, apiKey string, whenDone func()) {
	S := ATCStatus{}
	for {
		time.Sleep(time.Duration((240 + rand.Intn(120))) * time.Second) // 5 mins +/- up to 1 min
		S.getStatusFromATC(atcUrl, apiKey)
		log.Info().Bool("ADSB", S.Status.ADSB.Connected).Bool("MLAT", S.Status.MLAT.Connected).Msg("Connection Status")
	}
	whenDone()
}
