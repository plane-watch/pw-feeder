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
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (

	// mock ATC server testing scenarios
	MockServerTestScenarioWorking = iota
	MockServerTestScenarioBEASTOnly
	MockServerTestScenarioMLATOnly
	MockServerTestScenarioNoResponse
	MockServerTestScenarioBadRequest
	MockServerTestScenarioInvalidJSON
	MockServerTestScenarioServerError
)

var (
	TestFeederAPIKey = uuid.New()
)

func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.UnixDate})
}

func PrepMockATCServer(t *testing.T, testScenario int) *httptest.Server {

	// Thanks to: https://medium.com/zus-health/mocking-outbound-http-requests-in-go-youre-probably-doing-it-wrong-60373a38d2aa

	// prep test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		switch r.URL.Path {

		case fmt.Sprintf("/api/v1/feeders/%s/status.json", strings.ToLower(TestFeederAPIKey.String())):

			// check request
			assert.Equal(t, http.MethodGet, r.Method)

			// mock response
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

			// response code
			switch testScenario {
			case MockServerTestScenarioBadRequest:
				w.WriteHeader(http.StatusBadRequest)
			default:
				w.WriteHeader(http.StatusOK)
			}

			// response body
			switch testScenario {
			case MockServerTestScenarioInvalidJSON:
				w.Write([]byte(resp)[2:])
			case MockServerTestScenarioServerError:
				return
			default:
				w.Write([]byte(resp))
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

func TestGetStatusFromATC(t *testing.T) {

	t.Run("connection refused", func(t *testing.T) {
		// start test server
		testServer := PrepMockATCServer(t, MockServerTestScenarioBadRequest)
		testServer.Close()

		// test function
		S := ATCStatus{}
		err := S.getStatusFromATC(testServer.URL, TestFeederAPIKey.String())

		// ensure function behaves as expected
		require.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "connection refused"))

	})

	t.Run("bad request", func(t *testing.T) {
		// start test server
		testServer := PrepMockATCServer(t, MockServerTestScenarioBadRequest)
		t.Cleanup(func() {
			testServer.Close()
		})

		// test function
		S := ATCStatus{}
		err := S.getStatusFromATC(testServer.URL, TestFeederAPIKey.String())

		// ensure function behaves as expected
		require.Error(t, err)
		assert.Equal(t, ErrResponseNotOK.Error(), err.Error())
	})

	t.Run("server error", func(t *testing.T) {
		// start test server
		testServer := PrepMockATCServer(t, MockServerTestScenarioServerError)
		t.Cleanup(func() {
			testServer.Close()
		})

		// test function
		S := ATCStatus{}
		err := S.getStatusFromATC(testServer.URL, TestFeederAPIKey.String())

		// ensure function behaves as expected
		require.Error(t, err)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		// start test server
		testServer := PrepMockATCServer(t, MockServerTestScenarioInvalidJSON)
		t.Cleanup(func() {
			testServer.Close()
		})

		// test function
		S := ATCStatus{}
		err := S.getStatusFromATC(testServer.URL, TestFeederAPIKey.String())

		// ensure function behaves as expected
		require.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "invalid"))
	})

	t.Run("working BEAST", func(t *testing.T) {
		// start test server
		testServer := PrepMockATCServer(t, MockServerTestScenarioBEASTOnly)
		t.Cleanup(func() {
			testServer.Close()
		})

		// test function
		S := ATCStatus{}
		err := S.getStatusFromATC(testServer.URL, TestFeederAPIKey.String())

		// ensure function behaves as expected
		require.NoError(t, err)
		assert.Equal(t, "healthy", S.Status.ADSB.status)
		assert.Equal(t, true, S.Status.ADSB.Connected)
	})

	t.Run("working MLAT", func(t *testing.T) {
		// start test server
		testServer := PrepMockATCServer(t, MockServerTestScenarioMLATOnly)
		t.Cleanup(func() {
			testServer.Close()
		})

		// test function
		S := ATCStatus{}
		err := S.getStatusFromATC(testServer.URL, TestFeederAPIKey.String())

		// ensure function behaves as expected
		require.NoError(t, err)
		assert.Equal(t, "healthy", S.Status.MLAT.status)
		assert.Equal(t, true, S.Status.MLAT.Connected)
	})

	t.Run("working BEAST and MLAT", func(t *testing.T) {
		// start test server
		testServer := PrepMockATCServer(t, MockServerTestScenarioWorking)
		t.Cleanup(func() {
			testServer.Close()
		})

		// test function
		S := ATCStatus{}
		err := S.getStatusFromATC(testServer.URL, TestFeederAPIKey.String())

		// ensure function behaves as expected
		require.NoError(t, err)
		assert.Equal(t, "healthy", S.Status.ADSB.status)
		assert.Equal(t, "healthy", S.Status.MLAT.status)
		assert.Equal(t, true, S.Status.ADSB.Connected)
		assert.Equal(t, true, S.Status.MLAT.Connected)
	})
}

func TestStartStop(t *testing.T) {
	t.Run("working BEAST only", func(t *testing.T) {
		// start test server
		testServer := PrepMockATCServer(t, MockServerTestScenarioBEASTOnly)
		t.Cleanup(func() {
			testServer.Close()
		})

		// reduce random wait
		randSeconds = 1

		// prep parent context & waitgroup
		testCtx, _ := context.WithTimeout(context.Background(), time.Second*30)
		wg := sync.WaitGroup{}

		// start
		wg.Add(1)
		go func(t *testing.T) {
			defer wg.Done()
			Start(testCtx, testServer.URL, TestFeederAPIKey.String(), 60)
		}(t)

		// wait for some logging
		time.Sleep(time.Second * 10)

		// stop
		wg.Add(1)
		go func(t *testing.T) {
			defer wg.Done()
			Stop()
		}(t)

		// wait for everything to finish
		wg.Wait()

		fmt.Println(testCtx.Deadline())
		fmt.Println(testCtx.Err())

	})

	t.Run("working BEAST & MLAT", func(t *testing.T) {
		// start test server
		testServer := PrepMockATCServer(t, MockServerTestScenarioWorking)
		t.Cleanup(func() {
			testServer.Close()
		})

		// reduce random wait
		randSeconds = 1

		// prep parent context & waitgroup
		testCtx, _ := context.WithTimeout(context.Background(), time.Second*30)
		wg := sync.WaitGroup{}

		// start
		wg.Add(1)
		go func(t *testing.T) {
			defer wg.Done()
			Start(testCtx, testServer.URL, TestFeederAPIKey.String(), 60)
		}(t)

		// wait for some logging
		time.Sleep(time.Second * 10)

		// stop
		wg.Add(1)
		go func(t *testing.T) {
			defer wg.Done()
			Stop()
		}(t)

		// wait for everything to finish
		wg.Wait()

		fmt.Println(testCtx.Deadline())
		fmt.Println(testCtx.Err())

	})
}
