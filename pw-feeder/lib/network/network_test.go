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
