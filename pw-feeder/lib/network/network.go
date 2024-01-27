package network

import (
	"net"
	"time"

	"github.com/rs/zerolog/log"
)

// ConnectToHost connects to a host, returns a connection object
func ConnectToHost(name, addr string) (c net.Conn, err error) {

	// set log context
	logger := log.With().Str("name", name).Str("addr", addr).Logger()

	// prepare dialer with timeout
	d := net.Dialer{
		Timeout: 10 * time.Second,
	}

	// dial out
	c, err = d.Dial("tcp", addr)
	if err != nil {
		logger.Err(err).Msg("error establishing connection")
	}
	logger.Debug().Msg("endpoint connected")

	return c, err
}
