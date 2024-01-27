package connproxy

import (
	"context"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/nettest"
)

var (
	TestClientAPIKey = uuid.New()
)

func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.UnixDate})
}

func TestTunnelStats(t *testing.T) {
	ts := tunnelStats{}
	ts.incrementByteCounter(1, 2, 3, 4)
	bytesRxLocal, bytesTxLocal, bytesRxRemote, bytesTxRemote := ts.readStats()
	assert.Equal(t, bytesRxLocal, uint64(1))
	assert.Equal(t, bytesTxLocal, uint64(2))
	assert.Equal(t, bytesRxRemote, uint64(3))
	assert.Equal(t, bytesTxRemote, uint64(4))
}

func TestLogStats(t *testing.T) {
	ts := tunnelStats{}
	ts.incrementByteCounter(1, 2, 3, 4)
	wg := sync.WaitGroup{}
	testCtx, testCancel := context.WithCancel(context.Background())
	wg.Add(1)
	go func() {
		defer wg.Done()
		logStats(testCtx, &ts, "Test Protocol", time.Second)
	}()
	time.Sleep(time.Second * 5)
	testCancel()
	wg.Wait()
}

func TestDataMover(t *testing.T) {

	testBytes := []byte("Hello World! 1234567890")

	t.Run("NettoTLS working", func(t *testing.T) {
		connAIn, connAOut := net.Pipe()
		connBIn, connBOut := net.Pipe()

		ctx := context.Background()

		ts := tunnelStats{}
		wg := sync.WaitGroup{}
		waitRead := make(chan bool)

		wg.Add(1)
		go func() {
			defer wg.Done()
			dataMoverNettoTLS(ctx, connAOut, connBIn, &ts)
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := connAIn.Write(testBytes)
			require.NoError(t, err)
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			b := make([]byte, 1000)
			n, err := connBOut.Read(b)
			require.NoError(t, err)
			assert.Equal(t, testBytes, b[:n])
			waitRead <- true
		}()

		// wait for read to complete
		_ = <-waitRead

		// close connections
		_ = connAIn.Close()
		_ = connAOut.Close()
		_ = connBIn.Close()
		_ = connBOut.Close()

		// wait for everything to finish
		wg.Wait()
	})

	t.Run("NettoTLS context cancel", func(t *testing.T) {
		connAIn, connAOut := net.Pipe()
		connBIn, connBOut := net.Pipe()

		ctx, cancel := context.WithCancel(context.Background())

		ts := tunnelStats{}
		wg := sync.WaitGroup{}

		wg.Add(1)
		go func() {
			defer wg.Done()
			dataMoverNettoTLS(ctx, connAOut, connBIn, &ts)
		}()

		// context cancel
		cancel()

		// close connections
		_ = connAIn.Close()
		_ = connAOut.Close()
		_ = connBIn.Close()
		_ = connBOut.Close()

		// wait for everything to finish
		wg.Wait()
	})

	t.Run("TLStoNet context cancel", func(t *testing.T) {
		connAIn, connAOut := net.Pipe()
		connBIn, connBOut := net.Pipe()

		ctx, cancel := context.WithCancel(context.Background())

		ts := tunnelStats{}
		wg := sync.WaitGroup{}

		wg.Add(1)
		go func() {
			defer wg.Done()
			dataMoverTLStoNet(ctx, connAOut, connBIn, &ts)
		}()

		// context cancel
		cancel()

		// close connections
		_ = connAIn.Close()
		_ = connAOut.Close()
		_ = connBIn.Close()
		_ = connBOut.Close()

		// wait for everything to finish
		wg.Wait()
	})

	t.Run("TLStoNet working", func(t *testing.T) {
		connAIn, connAOut := net.Pipe()
		connBIn, connBOut := net.Pipe()

		ctx := context.Background()

		ts := tunnelStats{}
		wg := sync.WaitGroup{}
		waitRead := make(chan bool)

		wg.Add(1)
		go func() {
			defer wg.Done()
			dataMoverTLStoNet(ctx, connAOut, connBIn, &ts)
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := connAIn.Write(testBytes)
			require.NoError(t, err)
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			b := make([]byte, 1000)
			n, err := connBOut.Read(b)
			require.NoError(t, err)
			assert.Equal(t, testBytes, b[:n])
			waitRead <- true
		}()

		// wait for read to complete
		_ = <-waitRead

		// close connections
		_ = connBOut.Close()
		_ = connBIn.Close()
		_ = connAOut.Close()
		_ = connAIn.Close()

		// wait for everything to finish
		wg.Wait()
	})

	t.Run("dataMover working", func(t *testing.T) {
		connAIn, connAOut := net.Pipe()
		connBIn, connBOut := net.Pipe()

		wg := sync.WaitGroup{}
		waitRead := make(chan bool)

		wg.Add(1)
		go func() {
			defer wg.Done()
			bytesRead, bytesWritten, err := dataMover(connAOut, connBIn)
			require.NoError(t, err)
			assert.Equal(t, len(testBytes), bytesRead)
			assert.Equal(t, len(testBytes), bytesWritten)
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := connAIn.Write(testBytes)
			require.NoError(t, err)
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			b := make([]byte, 1000)
			n, err := connBOut.Read(b)
			require.NoError(t, err)
			assert.Equal(t, testBytes, b[:n])
			waitRead <- true
		}()

		// wait for read to complete
		_ = <-waitRead

		// close connections
		_ = connBOut.Close()
		_ = connBIn.Close()
		_ = connAOut.Close()
		_ = connAIn.Close()

		// wait for everything to finish
		wg.Wait()
	})

	t.Run("dataMover error writing", func(t *testing.T) {
		connAIn, connAOut := net.Pipe()
		connBIn, connBOut := net.Pipe()

		wg := sync.WaitGroup{}

		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := dataMover(connAOut, connBIn)
			require.Error(t, err)
		}()

		// introduce write error
		connBIn.Close()

		wg.Add(1)
		go func() {
			defer wg.Done()
			connAIn.Write(testBytes)
		}()

		// wait for everything to finish
		wg.Wait()

		// close connections
		_ = connBOut.Close()
		_ = connAOut.Close()
		_ = connAIn.Close()

	})
}

func TestProxyOutboundConnection(t *testing.T) {

	testData := []byte("Test BEAST data! 1234567890")

	// override func for testing
	connectToPlaneWatch_original := connectToPlaneWatch
	t.Cleanup(func() {
		connectToPlaneWatch = connectToPlaneWatch_original
	})
	connectToPlaneWatch = func(name string, addr string, sni string) (c net.Conn, err error) {
		return net.Dial("tcp4", addr)
	}

	// override vars for testing
	logStatsInterval_original := logStatsInterval
	errSleepTime_original := errSleepTime
	t.Cleanup(func() {
		logStatsInterval = logStatsInterval_original
		errSleepTime = errSleepTime_original

	})
	logStatsInterval = time.Second * 1
	errSleepTime = time.Second * 1

	t.Run("cannot connect to plane.watch endpoint", func(t *testing.T) {
		var err error

		ctx, cancel := context.WithCancel(context.Background())

		wg := sync.WaitGroup{}

		// mock plane.watch server listener
		nl, err := nettest.NewLocalListener("tcp4")
		require.NoError(t, err)

		// close mock plane.watch server listener to induce error
		nl.Close()

		// mock beast provider listener
		bp, err := nettest.NewLocalListener("tcp4")
		require.NoError(t, err)

		// start proxy
		wg.Add(1)
		go func() {
			defer wg.Done()
			ProxyBEASTConnection(ctx, "BEAST", bp.Addr().String(), nl.Addr().String(), TestClientAPIKey.String())
		}()

		// wait for a connection attempt
		t.Log("wait for a connection attempt")
		time.Sleep(time.Second * 10)

		// cancel context
		cancel()

		// wait for goroutines
		wg.Wait()

		// close listeners
		bp.Close()

	})

	t.Run("cannot connect to local endpoint", func(t *testing.T) {
		var err error

		ctx, cancel := context.WithCancel(context.Background())

		finishChan := make(chan bool)

		wg := sync.WaitGroup{}

		// mock plane.watch server listener
		nl, err := nettest.NewLocalListener("tcp4")
		require.NoError(t, err)

		// mock plane.watch server
		// accepts one connection, reads data, replies with the same data, closes the connection
		wg.Add(1)
		go func(t *testing.T) {
			defer wg.Done()

			t.Logf("mock plane.watch server listening on: %s", nl.Addr().String())
			c, err := nl.Accept()
			require.NoError(t, err, "mock plane.watch server accepting connection")
			t.Log("mock plane.watch server accepted connection")

			finishChan <- true
			c.Close()
		}(t)

		// mock beast provider listener
		bp, err := nettest.NewLocalListener("tcp4")
		require.NoError(t, err)

		// close beast provider to induce error
		bp.Close()

		// start proxy
		wg.Add(1)
		go func() {
			defer wg.Done()
			ProxyBEASTConnection(ctx, "BEAST", bp.Addr().String(), nl.Addr().String(), TestClientAPIKey.String())
		}()

		// wait for a connection attempt
		t.Log("wait for a connection attempt")
		time.Sleep(time.Second * 10)

		// cancel context
		cancel()

		// allow mock beast provider to close conn
		_ = <-finishChan

		// wait for goroutines
		wg.Wait()

		// close listeners
		nl.Close()

	})

	t.Run("working", func(t *testing.T) {
		var err error

		ctx, cancel := context.WithCancel(context.Background())

		finishChan := make(chan bool)

		wg := sync.WaitGroup{}

		// mock plane.watch server listener
		nl, err := nettest.NewLocalListener("tcp4")
		require.NoError(t, err)

		// mock plane.watch server
		// accepts one connection, reads data, replies with the same data, closes the connection
		wg.Add(1)
		go func(t *testing.T) {
			defer wg.Done()
			buf := make([]byte, 1000)

			t.Logf("mock plane.watch server listening on: %s", nl.Addr().String())
			c, err := nl.Accept()
			require.NoError(t, err, "mock plane.watch server accepting connection")
			t.Log("mock plane.watch server accepted connection")

			n, err := c.Read(buf)
			require.NoError(t, err, "mock plane.watch server reading from connection")
			t.Log("mock plane.watch server read data")
			assert.Equal(t, len(testData), n)
			assert.Equal(t, testData, buf[:n])

			n, err = c.Write(buf[:n])
			require.NoError(t, err, "mock plane.watch server writing to connection")
			t.Log("mock plane.watch server wrote data")
			assert.Equal(t, len(testData), n)

			err = c.Close()
			require.NoError(t, err, "mock plane.watch server closing connection")
			t.Log("mock plane.watch server closed connection")

			finishChan <- true
		}(t)

		// mock beast provider listener
		bp, err := nettest.NewLocalListener("tcp4")
		require.NoError(t, err)

		// mock beast provider
		// accepts one connection, writes data, reads reply, closes the connection
		wg.Add(1)
		go func(t *testing.T) {
			defer wg.Done()
			buf := make([]byte, 1000)

			t.Logf("mock beast provider listening on: %s", bp.Addr().String())
			c, err := bp.Accept()
			require.NoError(t, err, "mock beast provider accepting connection")
			t.Log("mock beast provider accepted connection")

			n, err := c.Write(testData)
			require.NoError(t, err, "mock beast provider writing to connection")
			t.Log("mock beast provider wrote data")
			assert.Equal(t, len(testData), n)

			n, err = c.Read(buf)
			require.NoError(t, err, "mock beast provider reading from connection")
			t.Log("mock beast provider read data")
			assert.Equal(t, len(testData), n)
			assert.Equal(t, testData, buf[:n])

			err = c.Close()
			require.NoError(t, err, "mock beast provider closing connection")
			t.Log("mock beast provider closed connection")

			finishChan <- true
		}(t)

		// start proxy
		wg.Add(1)
		go func() {
			defer wg.Done()
			ProxyBEASTConnection(ctx, "BEAST", bp.Addr().String(), nl.Addr().String(), TestClientAPIKey.String())
		}()

		// wait for data transfers
		_ = <-finishChan
		_ = <-finishChan

		// cancel context
		cancel()

		// wait for goroutines
		wg.Wait()

		// close listeners
		nl.Close()
		bp.Close()

	})

	t.Run("context cancel", func(t *testing.T) {
		var err error

		ctx, cancel := context.WithCancel(context.Background())

		finishChan := make(chan bool)

		wg := sync.WaitGroup{}

		// mock plane.watch server listener
		nl, err := nettest.NewLocalListener("tcp4")
		require.NoError(t, err)

		// mock plane.watch server
		// accepts one connection, reads data, replies with the same data, closes the connection
		wg.Add(1)
		go func(t *testing.T) {
			defer wg.Done()

			t.Logf("mock plane.watch server listening on: %s", nl.Addr().String())
			c, err := nl.Accept()
			require.NoError(t, err, "mock plane.watch server accepting connection")
			t.Log("mock plane.watch server accepted connection")

			<-finishChan

			err = c.Close()
			require.NoError(t, err, "mock plane.watch server closing connection")
			t.Log("mock plane.watch server closed connection")

		}(t)

		// mock beast provider listener
		bp, err := nettest.NewLocalListener("tcp4")
		require.NoError(t, err)

		// mock beast provider
		// accepts one connection, writes data, reads reply, closes the connection
		wg.Add(1)
		go func(t *testing.T) {
			defer wg.Done()

			t.Logf("mock beast provider listening on: %s", bp.Addr().String())
			c, err := bp.Accept()
			require.NoError(t, err, "mock beast provider accepting connection")
			t.Log("mock beast provider accepted connection")

			<-finishChan

			err = c.Close()
			require.NoError(t, err, "mock beast provider closing connection")
			t.Log("mock beast provider closed connection")
		}(t)

		// start proxy
		wg.Add(1)
		go func() {
			defer wg.Done()
			ProxyBEASTConnection(ctx, "BEAST", bp.Addr().String(), nl.Addr().String(), TestClientAPIKey.String())
			t.Log("ProxyOutboundConnection done")
			finishChan <- true
			finishChan <- true
		}()

		// wait for connections
		t.Log("sleeping for a bit")
		time.Sleep(time.Second * 1)

		// cancel context
		t.Log("cancelling context")
		cancel()

		// wait for goroutines
		t.Log("waiting for goroutines")
		wg.Wait()

		// close listeners
		nl.Close()
		bp.Close()
	})

	t.Run("terminate tunnel", func(t *testing.T) {
		var err error

		ctx, cancel := context.WithCancel(context.Background())

		finishChan := make(chan bool)

		wg := sync.WaitGroup{}

		// mock plane.watch server listener
		nl, err := nettest.NewLocalListener("tcp4")
		require.NoError(t, err)

		// mock plane.watch server
		// accepts one connection, reads data, replies with the same data, closes the connection
		wg.Add(1)
		go func(t *testing.T) {
			defer wg.Done()

			t.Logf("mock plane.watch server listening on: %s", nl.Addr().String())
			c, err := nl.Accept()
			require.NoError(t, err, "mock plane.watch server accepting connection")
			t.Log("mock plane.watch server accepted connection")

			<-finishChan

			err = c.Close()
			require.NoError(t, err, "mock plane.watch server closing connection")
			t.Log("mock plane.watch server closed connection")

		}(t)

		// mock beast provider listener
		bp, err := nettest.NewLocalListener("tcp4")
		require.NoError(t, err)

		// mock beast provider
		// accepts one connection, writes data, reads reply, closes the connection
		wg.Add(1)
		go func(t *testing.T) {
			defer wg.Done()

			t.Logf("mock beast provider listening on: %s", bp.Addr().String())
			c, err := bp.Accept()
			require.NoError(t, err, "mock beast provider accepting connection")
			t.Log("mock beast provider accepted connection")

			<-finishChan

			err = c.Close()
			require.NoError(t, err, "mock beast provider closing connection")
			t.Log("mock beast provider closed connection")
		}(t)

		// start proxy
		wg.Add(1)
		go func() {
			defer wg.Done()
			ProxyBEASTConnection(ctx, "BEAST", bp.Addr().String(), nl.Addr().String(), TestClientAPIKey.String())
			t.Log("ProxyOutboundConnection done")
		}()

		// wait for connections
		t.Log("sleeping for a bit")
		time.Sleep(time.Second * 1)

		// close connections
		finishChan <- true
		finishChan <- true

		// wait for connections
		t.Log("sleeping for a bit")
		time.Sleep(time.Second * 1)

		// cancel context
		t.Log("cancelling context")
		cancel()

		// wait for goroutines
		t.Log("waiting for goroutines")
		wg.Wait()

		// close listeners
		nl.Close()
		bp.Close()
	})

}

func TestProxyInboundConnection(t *testing.T) {

	testData := []byte("Test MLAT data! 1234567890")

	// override func for testing
	connectToPlaneWatch_original := connectToPlaneWatch
	t.Cleanup(func() {
		connectToPlaneWatch = connectToPlaneWatch_original
	})
	connectToPlaneWatch = func(name string, addr string, sni string) (c net.Conn, err error) {
		return net.DialTimeout("tcp4", addr, time.Second*10)
	}

	// override vars for testing
	logStatsInterval_original := logStatsInterval
	errSleepTime_original := errSleepTime
	t.Cleanup(func() {
		logStatsInterval = logStatsInterval_original
		errSleepTime = errSleepTime_original

	})
	logStatsInterval = time.Second * 1
	errSleepTime = time.Second * 1

	t.Run("could not accept connection", func(t *testing.T) {
		var err error

		ctx, cancel := context.WithCancel(context.Background())

		wg := sync.WaitGroup{}

		// mock plane.watch server listener
		nl, err := nettest.NewLocalListener("tcp4")
		require.NoError(t, err)
		defer nl.Close()

		// mock mlat provider listener
		mp, err := nettest.NewLocalListener("tcp4")
		require.NoError(t, err)
		defer mp.Close()

		// close listener to induce error
		mp.Close()

		// start proxy
		wg.Add(1)
		go func(t *testing.T) {
			defer wg.Done()
			ProxyMLATConnection(ctx, "MLAT", mp, nl.Addr().String(), TestClientAPIKey.String())
		}(t)

		// wait for connection attempts
		time.Sleep(time.Second * 1)

		// cancel context
		cancel()

		// wait for goroutines
		wg.Wait()

	})

	t.Run("could not connect to plane.watch", func(t *testing.T) {
		var err error

		ctx, cancel := context.WithCancel(context.Background())

		wg := sync.WaitGroup{}

		stopChan := make(chan bool)

		// mock plane.watch server listener
		nl, err := nettest.NewLocalListener("tcp4")
		require.NoError(t, err)
		defer nl.Close()

		// close mock plane.watch server listener to induce error
		nl.Close()

		// mlat listener
		mp, err := nettest.NewLocalListener("tcp4")
		require.NoError(t, err)
		defer mp.Close()

		// start proxy
		wg.Add(1)
		go func(t *testing.T) {
			defer wg.Done()
			ProxyMLATConnection(ctx, "MLAT", mp, nl.Addr().String(), TestClientAPIKey.String())
		}(t)

		// mock mlat-client
		wg.Add(1)
		go func(t *testing.T) {
			defer wg.Done()

			// connect
			net.Dial("tcp4", mp.Addr().String())

			// wait for tests
			<-stopChan

		}(t)

		// wait for connection attempts
		time.Sleep(time.Second * 1)

		stopChan <- true

		// cancel context
		cancel()

		// wait for goroutines
		wg.Wait()
	})

	t.Run("working with context cancel", func(t *testing.T) {
		var err error

		ctx, cancel := context.WithCancel(context.Background())

		wg := sync.WaitGroup{}

		finishChan := make(chan bool)

		// mock plane.watch server listener
		nl, err := nettest.NewLocalListener("tcp4")
		require.NoError(t, err)
		defer nl.Close()

		// mock plane.watch server
		// accepts one connection, reads data, replies with the same data, closes the connection
		wg.Add(1)
		go func(t *testing.T) {
			defer wg.Done()
			buf := make([]byte, 1000)

			t.Logf("mock plane.watch server listening on: %s", nl.Addr().String())
			c, err := nl.Accept()
			require.NoError(t, err, "mock plane.watch server accepting connection")
			t.Log("mock plane.watch server accepted connection")

			n, err := c.Read(buf)
			require.NoError(t, err, "mock plane.watch server reading from connection")
			t.Log("mock plane.watch server read data")
			assert.Equal(t, len(testData), n)
			assert.Equal(t, testData, buf[:n])

			n, err = c.Write(buf[:n])
			require.NoError(t, err, "mock plane.watch server writing to connection")
			t.Log("mock plane.watch server wrote data")
			assert.Equal(t, len(testData), n)

			err = c.Close()
			require.NoError(t, err, "mock plane.watch server closing connection")
			t.Log("mock plane.watch server closed connection")

			finishChan <- true
		}(t)

		// mlat listener
		mp, err := nettest.NewLocalListener("tcp4")
		require.NoError(t, err)
		defer mp.Close()

		// start proxy
		wg.Add(1)
		go func(t *testing.T) {
			defer wg.Done()
			ProxyMLATConnection(ctx, "MLAT", mp, nl.Addr().String(), TestClientAPIKey.String())
		}(t)

		// mock mlat-client
		wg.Add(1)
		go func(t *testing.T) {
			defer wg.Done()
			buf := make([]byte, 1000)

			// connect
			c, err := net.Dial("tcp4", mp.Addr().String())
			require.NoError(t, err)

			// write data
			n, err := c.Write(testData)
			require.NoError(t, err, "mock mlat-client writing to connection")
			assert.Equal(t, len(testData), n)

			// read data
			n, err = c.Read(buf)
			require.NoError(t, err, "mock mlat-client reading from connection")
			assert.Equal(t, len(testData), n)
			assert.Equal(t, testData, buf[:n])

			// close conn
			err = c.Close()
			require.NoError(t, err, "mock mlat-client closing connection")

			finishChan <- true
		}(t)

		// wait for connection attempts
		time.Sleep(time.Second * 1)

		// wait for tests
		<-finishChan
		<-finishChan

		// cancel context
		cancel()

		// wait for goroutines
		wg.Wait()

		nl.Close()
	})

	t.Run("working with full loop", func(t *testing.T) {
		var err error

		ctx, cancel := context.WithCancel(context.Background())

		wg := sync.WaitGroup{}

		finishChan := make(chan bool)

		// mock plane.watch server listener
		nl, err := nettest.NewLocalListener("tcp4")
		require.NoError(t, err)

		// mock plane.watch server
		// accepts one connection, reads data, replies with the same data, closes the connection
		wg.Add(1)
		go func(t *testing.T) {
			defer wg.Done()
			buf := make([]byte, 1000)

			for i := 0; i <= 2; i++ {

				t.Logf("mock plane.watch server listening on: %s", nl.Addr().String())
				c, err := nl.Accept()
				require.NoError(t, err, "mock plane.watch server accepting connection")
				t.Log("mock plane.watch server accepted connection")

				n, err := c.Read(buf)
				require.NoError(t, err, "mock plane.watch server reading from connection")
				t.Log("mock plane.watch server read data")
				assert.Equal(t, len(testData), n)
				assert.Equal(t, testData, buf[:n])

				n, err = c.Write(buf[:n])
				require.NoError(t, err, "mock plane.watch server writing to connection")
				t.Log("mock plane.watch server wrote data")
				assert.Equal(t, len(testData), n)

				err = c.Close()
				require.NoError(t, err, "mock plane.watch server closing connection")
				t.Log("mock plane.watch server closed connection")

			}

			finishChan <- true
		}(t)

		// mlat listener
		mp, err := nettest.NewLocalListener("tcp4")
		require.NoError(t, err)
		defer mp.Close()

		// start proxy
		wg.Add(1)
		go func(t *testing.T) {
			defer wg.Done()
			ProxyMLATConnection(ctx, "MLAT", mp, nl.Addr().String(), TestClientAPIKey.String())
		}(t)

		// mock mlat-client
		wg.Add(1)
		go func(t *testing.T) {
			defer wg.Done()
			buf := make([]byte, 1000)

			for i := 0; i <= 2; i++ {

				// connect
				t.Log("mock mlat-client attempting connection")
				c, err := net.Dial("tcp4", mp.Addr().String())
				require.NoError(t, err)

				// write data
				t.Log("mock mlat-client writing data")
				n, err := c.Write(testData)
				require.NoError(t, err, "mock mlat-client writing to connection")
				assert.Equal(t, len(testData), n)

				// read data
				t.Log("mock mlat-client reading data")
				n, err = c.Read(buf)
				require.NoError(t, err, "mock mlat-client reading from connection")
				assert.Equal(t, len(testData), n)
				assert.Equal(t, testData, buf[:n])

				// close conn
				t.Log("mock mlat-client closing connection")
				err = c.Close()
				require.NoError(t, err, "mock mlat-client closing connection")

			}

			finishChan <- true
		}(t)

		// wait for tests
		<-finishChan
		<-finishChan

		// close server
		nl.Close()

		// cancel context
		cancel()

		// wait for goroutines
		wg.Wait()

	})

}
