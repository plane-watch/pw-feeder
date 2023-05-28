package main

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTunnelStats(t *testing.T) {
	x := tunnelStats{}

	x.incrementByteCounter(1, 2, 3, 4)

	bytesRxLocal, bytesTxLocal, bytesRxRemote, bytesTxRemote := x.readStats()

	assert.Equal(t, bytesRxLocal, uint64(1))
	assert.Equal(t, bytesTxLocal, uint64(2))
	assert.Equal(t, bytesRxRemote, uint64(3))
	assert.Equal(t, bytesTxRemote, uint64(4))

}

func TestTunnelOutboundConnection(t *testing.T) {

	testDataIn := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 0}
	testDataOut := make([]byte, 10)
	dataIn := make(chan []byte)
	dataOut := make(chan []byte)
	wg := sync.WaitGroup{}

	// start test TLS server
	t.Log("starting test SSL server")
	wg.Add(1)
	go startTLSServer(t, &wg, "127.0.0.1:32346")
	t.Log("waiting for test SSL server to start")
	wg.Wait()

	// start test TCP client
	wg.Add(1)
	go startTCPServer(t, &wg, "127.0.0.1:32347", dataIn, dataOut)
	t.Log("waiting for test TCP server to start")
	wg.Wait()

	// test tunnelOutboundConnection
	t.Log("starting tunnelOutboundConnection")
	go tunnelOutboundConnection("test", "127.0.0.1:32347", "127.0.0.1:32346", "A30101A1-30AA-4DFD-9B91-7168BE952A73", func() {})

	// send data through the system
	t.Log("send data through the system")
	dataIn <- testDataIn
	testDataOut = <-dataOut

	// make sure data in = data out
	assert.Equal(t, testDataIn, testDataOut)

}

func TestTunnelInboundConnection(t *testing.T) {

	testDataIn := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 0}
	testDataOut := make([]byte, 10)
	dataIn := make(chan []byte)
	dataOut := make(chan []byte)
	wg := sync.WaitGroup{}

	// start test TLS server
	t.Log("starting test SSL server")
	wg.Add(1)
	go startTLSServer(t, &wg, "127.0.0.1:32348")
	t.Log("waiting for test SSL server to start")
	wg.Wait()

	// test tunnelOutboundConnection
	t.Log("starting tunnelInboundConnection")
	go tunnelInboundConnection("test", "127.0.0.1:32349", "127.0.0.1:32348", "A30101A1-30AA-4DFD-9B91-7168BE952A73", func() {})

	// start test TCP client
	go startTCPClient(t, "127.0.0.1:32349", dataIn, dataOut)

	// send data through the system
	t.Log("send data through the system")
	dataIn <- testDataIn
	testDataOut = <-dataOut

	// make sure data in = data out
	assert.Equal(t, testDataIn, testDataOut)

}
