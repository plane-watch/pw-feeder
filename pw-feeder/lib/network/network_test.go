package network

import (
	"os"
	"testing"
	"time"

	"golang.org/x/net/nettest"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
)

// init configures console logging for the package tests.
func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.UnixDate})
}

// TestConnectToHost verifies successful and failed TCP connection attempts.
func TestConnectToHost(t *testing.T) {

	t.Run("working", func(t *testing.T) {
		// Set up a test listener.
		tl, err := nettest.NewLocalListener("tcp")
		require.NoError(t, err)
		defer func() {
			_ = tl.Close()
		}()

		// Attempt to connect.
		c, err := ConnectToHost("test", tl.Addr().String())
		require.NoError(t, err)
		defer func() {
			_ = c.Close()
		}()
	})

	t.Run("error", func(t *testing.T) {
		// Set up a test listener.
		tl, err := nettest.NewLocalListener("tcp")
		require.NoError(t, err)

		// Save the listener address.
		testAddr := tl.Addr().String()

		// Close the test listener to induce an error.
		_ = tl.Close()

		// Attempt to connect.
		_, err = ConnectToHost("test", testAddr)
		require.Error(t, err)
	})

}
