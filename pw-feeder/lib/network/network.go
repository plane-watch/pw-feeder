// Copyright (C) 2024 Plane Watch
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This file is part of pw-feeder.
//
// pw-feeder is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// pw-feeder is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with pw-feeder. If not, see <https://www.gnu.org/licenses/>.

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
