package main

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStunnelConnect(t *testing.T) {

	// start test server
	t.Log("starting test SSL server")
	wg := sync.WaitGroup{}
	wg.Add(1)
	go startTLSServer(t, &wg, "127.0.0.1:32345")
	t.Log("waiting for test SSL server to start")
	wg.Wait()

	// test stunnelConnect
	t.Log("test stunnelConnect")
	c, err := stunnelConnect("test", "127.0.0.1:32345", "7BE5F7FD-BA97-4280-9B15-4F0746D875DA")
	if err != nil {
		t.Error(err)
	}
	assert.NoError(t, err)
	defer c.Close()

}
