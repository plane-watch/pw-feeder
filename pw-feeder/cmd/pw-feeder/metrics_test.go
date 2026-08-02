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

package main

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetricsServiceDisabled(t *testing.T) {
	service, err := prepareMetrics(context.Background(), feederConfig{})
	require.NoError(t, err)
	assert.Nil(t, service.Registerer())
	assert.Nil(t, service.Errors())
	require.NoError(t, service.Start())
	require.NoError(t, service.Shutdown(context.Background()))
}

func TestMetricsServiceLifecycle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	service, err := prepareMetrics(ctx, feederConfig{
		metricsEnabled: true,
		metricsAddress: "127.0.0.1:0",
	})
	require.NoError(t, err)
	assert.NotNil(t, service.Registerer())
	assert.NotNil(t, service.Errors())
	require.NoError(t, service.Start())

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
	defer shutdownCancel()
	require.NoError(t, service.Shutdown(shutdownCtx))
}

func TestMetricsServiceReportsBindErrors(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() {
		_ = listener.Close()
	}()

	service, err := prepareMetrics(context.Background(), feederConfig{
		metricsEnabled: true,
		metricsAddress: listener.Addr().String(),
	})
	require.NoError(t, err)

	err = service.Start()
	require.Error(t, err)
	assert.ErrorContains(t, err, "could not start metrics listener")
}
