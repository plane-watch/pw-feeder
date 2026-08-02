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
