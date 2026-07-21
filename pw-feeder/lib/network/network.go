package network

import (
	"net"
	"time"

	"github.com/rs/zerolog/log"
)

// ConnectToHost establishes a TCP connection to addr using the supplied name
// for log context.
func ConnectToHost(name, addr string) (c net.Conn, err error) {

	// Add the connection details to the logger context.
	logger := log.With().Str("name", name).Str("addr", addr).Logger()

	// Prepare a dialer with a connection timeout.
	d := net.Dialer{
		Timeout: 10 * time.Second,
	}

	// Dial the remote endpoint.
	c, err = d.Dial("tcp", addr)
	if err != nil {
		logger.Err(err).Msg("error establishing connection")
	}
	logger.Debug().Msg("endpoint connected")

	return c, err
}
