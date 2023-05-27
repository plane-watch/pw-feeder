package main

import (
	"testing"

	"golang.org/x/net/nettest"

	"github.com/stretchr/testify/assert"
)

func TestConnectToHost(t *testing.T) {

	// set up test listener
	tl, err := nettest.NewLocalListener("tcp")
	if err != nil {
		t.Error(err)
	}
	assert.NoError(t, err)
	defer tl.Close()

	// attempt to connect
	c, err := connectToHost("test", tl.Addr().String())
	if err != nil {
		t.Error(err)
	}
	assert.NoError(t, err)
	defer c.Close()

}
