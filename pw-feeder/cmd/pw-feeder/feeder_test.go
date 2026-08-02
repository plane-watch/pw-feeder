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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWaitForShutdownContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.NoError(t, waitForShutdown(ctx, nil))
}

func TestWaitForShutdownMetricsError(t *testing.T) {
	wantErr := errors.New("metrics failed")
	metricsErrors := make(chan error, 1)
	metricsErrors <- wantErr

	err := waitForShutdown(context.Background(), metricsErrors)
	require.Error(t, err)
	assert.ErrorIs(t, err, wantErr)
}

func TestPrepareMLATListenerDisabled(t *testing.T) {
	listener, err := prepareMLATListener(feederConfig{})
	require.NoError(t, err)
	assert.Nil(t, listener)
}

func TestPrepareMLATListenerEnabled(t *testing.T) {
	listener, err := prepareMLATListener(feederConfig{
		mlatEnabled: true,
		mlatListen:  "127.0.0.1:0",
	})
	require.NoError(t, err)
	require.NotNil(t, listener)
	require.NoError(t, listener.Close())
}
