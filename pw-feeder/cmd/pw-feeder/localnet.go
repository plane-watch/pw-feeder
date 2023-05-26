package main

import (
	"net"
	"time"

	"github.com/rs/zerolog/log"
)

func connectToHost(name, addr string) (c net.Conn, err error) {

	logger := log.With().Str("name", name).Str("addr", addr).Logger()

	// establish connection to local remote host
	d := net.Dialer{
		Timeout: 10 * time.Second,
	}
	c, err = d.Dial("tcp", addr)
	if err != nil {
		logger.Err(err).Msg("error dialling")
	}

	logger.Debug().Msg("endpoint connected")

	return c, err
}
