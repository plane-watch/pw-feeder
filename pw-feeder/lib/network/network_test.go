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

func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.UnixDate})
}

func TestConnectToHost(t *testing.T) {

	t.Run("working", func(t *testing.T) {
		// set up test listener
		tl, err := nettest.NewLocalListener("tcp")
		require.NoError(t, err)
		defer func() {
			_ = tl.Close()
		}()

		// attempt to connect
		c, err := ConnectToHost("test", tl.Addr().String())
		require.NoError(t, err)
		defer func() {
			_ = c.Close()
		}()
	})

	t.Run("error", func(t *testing.T) {
		// set up test listener
		tl, err := nettest.NewLocalListener("tcp")
		require.NoError(t, err)

		// get address
		testAddr := tl.Addr().String()

		// close test listener (to induce error)
		_ = tl.Close()

		// attempt to connect
		_, err = ConnectToHost("test", testAddr)
		require.Error(t, err)
	})

}
