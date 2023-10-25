package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"
)

type ATCStatus struct {
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
	fmt.Println(res.Body)

}

func initStatusUpdater(atcUrl, apiKey string, whenDone func()) {
	S := ATCStatus{}
	for {
		S.getStatusFromATC(atcUrl, apiKey)
		time.Sleep(time.Duration((540 + rand.Intn(120))) * time.Second) // 10 mins +/- up to 1 min
	}
	whenDone()
}
