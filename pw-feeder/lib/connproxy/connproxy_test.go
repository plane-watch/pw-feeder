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
	// TestClientAPIKey identifies the client used by proxy tests.
	TestClientAPIKey = uuid.New()
)

// init configures console logging for the package tests.
func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.UnixDate})
}

// TestTunnelStats verifies atomic updates and reads of tunnel byte counters.
func TestTunnelStats(t *testing.T) {
	ts := tunnelStats{}
	ts.incrementByteCounter(1, 2, 3, 4)
	bytesRxLocal, bytesTxLocal, bytesRxRemote, bytesTxRemote := ts.readStats()
	assert.Equal(t, bytesRxLocal, uint64(1))
	assert.Equal(t, bytesTxLocal, uint64(2))
	assert.Equal(t, bytesRxRemote, uint64(3))
	assert.Equal(t, bytesTxRemote, uint64(4))
}

// TestLogStats verifies that statistics logging stops when its context is cancelled.
func TestLogStats(t *testing.T) {
	ts := tunnelStats{}
	ts.incrementByteCounter(1, 2, 3, 4)
	wg := sync.WaitGroup{}
	testCtx, testCancel := context.WithCancel(context.Background())
	wg.Go(func() {
		logStats(testCtx, &ts, "Test Protocol", time.Second)
	})
	time.Sleep(time.Second * 5)
	testCancel()
	wg.Wait()
}

// TestDataMover verifies bidirectional data transfer, cancellation, and errors.
func TestDataMover(t *testing.T) {

	logger := log.With().Caller().Logger()

	testBytes := []byte("Hello World! 1234567890")

	t.Run("Net to TLS working", func(t *testing.T) {
		connAIn, connAOut := net.Pipe()
		connBIn, connBOut := net.Pipe()

		ctx := context.Background()

		ts := tunnelStats{}
		wg := sync.WaitGroup{}
		waitRead := make(chan bool)

		wg.Go(func() {
			dataMoverNettoTLS(ctx, connAOut, connBIn, &ts, logger)
		})

		wg.Go(func() {
			_, err := connAIn.Write(testBytes)
			require.NoError(t, err)
		})

		wg.Go(func() {
			b := make([]byte, 1000)
			n, err := connBOut.Read(b)
			require.NoError(t, err)
			assert.Equal(t, testBytes, b[:n])
			waitRead <- true
		})

		// Wait for the read to complete.
		_ = <-waitRead

		// Close the connections.
		_ = connAIn.Close()
		_ = connAOut.Close()
		_ = connBIn.Close()
		_ = connBOut.Close()

		// Wait for all goroutines to finish.
		wg.Wait()
	})

	t.Run("NettoTLS context cancel", func(t *testing.T) {
		connAIn, connAOut := net.Pipe()
		connBIn, connBOut := net.Pipe()

		ctx, cancel := context.WithCancel(context.Background())

		ts := tunnelStats{}
		wg := sync.WaitGroup{}

		wg.Go(func() {
			dataMoverNettoTLS(ctx, connAOut, connBIn, &ts, logger)
		})

		// Cancel the context.
		cancel()

		// Close the connections.
		_ = connAIn.Close()
		_ = connAOut.Close()
		_ = connBIn.Close()
		_ = connBOut.Close()

		// Wait for all goroutines to finish.
		wg.Wait()
	})

	t.Run("TLS to Net context cancel", func(t *testing.T) {
		connAIn, connAOut := net.Pipe()
		connBIn, connBOut := net.Pipe()

		ctx, cancel := context.WithCancel(context.Background())

		ts := tunnelStats{}
		wg := sync.WaitGroup{}

		wg.Go(func() {
			dataMoverTLStoNet(ctx, connAOut, connBIn, &ts, logger)
		})

		// Cancel the context.
		cancel()

		// Close the connections.
		_ = connAIn.Close()
		_ = connAOut.Close()
		_ = connBIn.Close()
		_ = connBOut.Close()

		// Wait for all goroutines to finish.
		wg.Wait()
	})

	t.Run("TLS to Net working", func(t *testing.T) {
		connAIn, connAOut := net.Pipe()
		connBIn, connBOut := net.Pipe()

		ctx := context.Background()

		ts := tunnelStats{}
		wg := sync.WaitGroup{}
		waitRead := make(chan bool)

		wg.Go(func() {
			dataMoverTLStoNet(ctx, connAOut, connBIn, &ts, logger)
		})

		wg.Go(func() {
			_, err := connAIn.Write(testBytes)
			require.NoError(t, err)
		})

		wg.Go(func() {
			b := make([]byte, 1000)
			n, err := connBOut.Read(b)
			require.NoError(t, err)
			assert.Equal(t, testBytes, b[:n])
			waitRead <- true
		})

		// Wait for the read to complete.
		_ = <-waitRead

		// Close the connections.
		_ = connBOut.Close()
		_ = connBIn.Close()
		_ = connAOut.Close()
		_ = connAIn.Close()

		// Wait for all goroutines to finish.
		wg.Wait()
	})

	t.Run("dataMover working", func(t *testing.T) {
		connAIn, connAOut := net.Pipe()
		connBIn, connBOut := net.Pipe()

		wg := sync.WaitGroup{}
		waitRead := make(chan bool)

		wg.Go(func() {
			buf := make([]byte, dataMoverBufferSize)
			bytesRead, bytesWritten, err := dataMover(connAOut, connBIn, buf, logger)
			require.NoError(t, err)
			assert.Equal(t, len(testBytes), bytesRead)
			assert.Equal(t, len(testBytes), bytesWritten)
		})

		wg.Go(func() {
			_, err := connAIn.Write(testBytes)
			require.NoError(t, err)
		})

		wg.Go(func() {
			b := make([]byte, 1000)
			n, err := connBOut.Read(b)
			require.NoError(t, err)
			assert.Equal(t, testBytes, b[:n])
			waitRead <- true
		})

		// Wait for the read to complete.
		_ = <-waitRead

		// Close the connections.
		_ = connBOut.Close()
		_ = connBIn.Close()
		_ = connAOut.Close()
		_ = connAIn.Close()

		// Wait for all goroutines to finish.
		wg.Wait()
	})

	t.Run("dataMover error writing", func(t *testing.T) {
		connAIn, connAOut := net.Pipe()
		connBIn, connBOut := net.Pipe()

		wg := sync.WaitGroup{}

		wg.Go(func() {
			buf := make([]byte, dataMoverBufferSize)
			_, _, err := dataMover(connAOut, connBIn, buf, logger)
			require.Error(t, err)
		})

		// Close the destination to induce a write error.
		_ = connBIn.Close()

		wg.Go(func() {
			_, _ = connAIn.Write(testBytes)
		})

		// Wait for all goroutines to finish.
		wg.Wait()

		// Close the remaining connections.
		_ = connBOut.Close()
		_ = connAOut.Close()
		_ = connAIn.Close()

	})
}

// TestProxyOutboundConnection verifies BEAST proxy connections, transfers,
// retries, cancellation, and cleanup.
func TestProxyOutboundConnection(t *testing.T) {

	testData := []byte("Test BEAST data! 1234567890")

	// Replace the remote connector for testing.
	connectToPlaneWatchOriginal := connectToPlaneWatch
	t.Cleanup(func() {
		connectToPlaneWatch = connectToPlaneWatchOriginal
	})
	connectToPlaneWatch = func(name, addr, sni string, insecure bool) (c net.Conn, err error) {
		return net.Dial("tcp4", addr)
	}

	// Reduce test timing intervals.
	logStatsIntervalOriginal := logStatsInterval
	errSleepTimeOriginal := errSleepTime
	t.Cleanup(func() {
		logStatsInterval = logStatsIntervalOriginal
		errSleepTime = errSleepTimeOriginal
	})
	logStatsInterval = time.Second * 1
	errSleepTime = time.Second * 1

	t.Run("cannot connect to plane.watch endpoint", func(t *testing.T) {
		var err error

		ctx, cancel := context.WithCancel(context.Background())

		// Create the mock plane.watch listener.
		nl, err := nettest.NewLocalListener("tcp4")
		require.NoError(t, err)

		// Close the plane.watch listener to induce an error.
		_ = nl.Close()

		// Create the mock BEAST provider listener.
		bp, err := nettest.NewLocalListener("tcp4")
		require.NoError(t, err)
		defer func() {
			_ = bp.Close()
		}()

		// Start the proxy.
		wg := sync.WaitGroup{}
		wg.Go(func() {
			ProxyBEASTConnection(ctx, "BEAST", bp.Addr().String(), nl.Addr().String(), TestClientAPIKey.String(), false, nil)
		})

		// Wait for a connection attempt.
		t.Log("wait for a connection attempt")
		time.Sleep(time.Second * 10)

		// Cancel the context.
		cancel()

		// Wait for all goroutines to finish.
		wg.Wait()
	})

	t.Run("cannot connect to local endpoint", func(t *testing.T) {
		var err error

		ctx, cancel := context.WithCancel(context.Background())

		wg := sync.WaitGroup{}

		// Create the mock plane.watch listener.
		nl, err := nettest.NewLocalListener("tcp4")
		require.NoError(t, err)
		defer func() {
			_ = nl.Close()
		}()

		// Start a mock plane.watch server that accepts and closes one connection.
		wg.Go(func() {
			t.Logf("mock plane.watch server listening on: %s", nl.Addr().String())
			_ = nl.(*net.TCPListener).SetDeadline(time.Now().Add(time.Second * 5))
			c, err := nl.Accept()
			if err == nil {
				_ = c.Close()
			}
		})

		// Create the mock BEAST provider listener.
		bp, err := nettest.NewLocalListener("tcp4")
		require.NoError(t, err)

		// Close the BEAST provider to induce an error.
		_ = bp.Close()

		// Start the proxy.
		wg.Go(func() {
			ProxyBEASTConnection(ctx, "BEAST", bp.Addr().String(), nl.Addr().String(), TestClientAPIKey.String(), false, nil)
		})

		// Wait for a connection attempt.
		t.Log("wait for a connection attempt")
		time.Sleep(time.Second * 10)

		// Cancel the context.
		cancel()

		// Wait for all goroutines to finish.
		wg.Wait()
	})

	t.Run("working", func(t *testing.T) {
		var err error

		ctx, cancel := context.WithCancel(context.Background())

		finishChan := make(chan bool)

		wg := sync.WaitGroup{}

		// Create the mock plane.watch listener.
		nl, err := nettest.NewLocalListener("tcp4")
		require.NoError(t, err)
		defer func() {
			_ = nl.Close()
		}()

		// Start a mock plane.watch echo server for one connection.
		wg.Go(func() {
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
		})

		// Create the mock BEAST provider listener.
		bp, err := nettest.NewLocalListener("tcp4")
		require.NoError(t, err)
		defer func() {
			_ = bp.Close()
		}()

		// Start a mock BEAST provider for one connection.
		wg.Go(func() {
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
		})

		// Start the proxy.
		wg.Go(func() {
			ProxyBEASTConnection(ctx, "BEAST", bp.Addr().String(), nl.Addr().String(), TestClientAPIKey.String(), false, nil)
		})

		// Wait for both data transfers.
		_ = <-finishChan
		_ = <-finishChan

		// Cancel the context.
		cancel()

		// Wait for all goroutines to finish.
		wg.Wait()
	})

	t.Run("context cancel", func(t *testing.T) {
		var err error

		ctx, cancel := context.WithCancel(context.Background())

		finishChan := make(chan bool)

		wg := sync.WaitGroup{}

		// Create the mock plane.watch listener.
		nl, err := nettest.NewLocalListener("tcp4")
		require.NoError(t, err)
		defer func() {
			_ = nl.Close()
		}()

		// Start a mock plane.watch server for one connection.
		wg.Go(func() {
			t.Logf("mock plane.watch server listening on: %s", nl.Addr().String())
			c, err := nl.Accept()
			require.NoError(t, err, "mock plane.watch server accepting connection")
			t.Log("mock plane.watch server accepted connection")

			<-finishChan

			err = c.Close()
			require.NoError(t, err, "mock plane.watch server closing connection")
			t.Log("mock plane.watch server closed connection")

		})

		// Create the mock BEAST provider listener.
		bp, err := nettest.NewLocalListener("tcp4")
		require.NoError(t, err)
		defer func() {
			_ = bp.Close()
		}()

		// Start a mock BEAST provider for one connection.
		wg.Go(func() {
			t.Logf("mock beast provider listening on: %s", bp.Addr().String())
			c, err := bp.Accept()
			require.NoError(t, err, "mock beast provider accepting connection")
			t.Log("mock beast provider accepted connection")

			<-finishChan

			err = c.Close()
			require.NoError(t, err, "mock beast provider closing connection")
			t.Log("mock beast provider closed connection")
		})

		// Start the proxy.
		wg.Go(func() {
			ProxyBEASTConnection(ctx, "BEAST", bp.Addr().String(), nl.Addr().String(), TestClientAPIKey.String(), false, nil)
			t.Log("ProxyOutboundConnection done")
			finishChan <- true
			finishChan <- true
		})

		// Wait for the connections to be established.
		t.Log("sleeping for a bit")
		time.Sleep(time.Second * 1)

		// Cancel the context.
		t.Log("cancelling context")
		cancel()

		// Wait for all goroutines to finish.
		t.Log("waiting for goroutines")
		wg.Wait()
	})

	t.Run("terminate tunnel", func(t *testing.T) {
		var err error

		ctx, cancel := context.WithCancel(context.Background())

		finishChan := make(chan bool)

		wg := sync.WaitGroup{}

		// Create the mock plane.watch listener.
		nl, err := nettest.NewLocalListener("tcp4")
		require.NoError(t, err)
		defer func() {
			_ = nl.Close()
		}()

		// Start a mock plane.watch server for one connection.
		wg.Go(func() {
			t.Logf("mock plane.watch server listening on: %s", nl.Addr().String())
			c, err := nl.Accept()
			require.NoError(t, err, "mock plane.watch server accepting connection")
			t.Log("mock plane.watch server accepted connection")

			<-finishChan

			err = c.Close()
			require.NoError(t, err, "mock plane.watch server closing connection")
			t.Log("mock plane.watch server closed connection")

		})

		// Create the mock BEAST provider listener.
		bp, err := nettest.NewLocalListener("tcp4")
		require.NoError(t, err)
		defer func() {
			_ = bp.Close()
		}()

		// Start a mock BEAST provider for one connection.
		wg.Go(func() {
			t.Logf("mock beast provider listening on: %s", bp.Addr().String())
			c, err := bp.Accept()
			require.NoError(t, err, "mock beast provider accepting connection")
			t.Log("mock beast provider accepted connection")

			<-finishChan

			err = c.Close()
			require.NoError(t, err, "mock beast provider closing connection")
			t.Log("mock beast provider closed connection")
		})

		// Start the proxy.
		wg.Go(func() {
			ProxyBEASTConnection(ctx, "BEAST", bp.Addr().String(), nl.Addr().String(), TestClientAPIKey.String(), false, nil)
			t.Log("ProxyOutboundConnection done")
		})

		// Wait for the connections to be established.
		t.Log("sleeping for a bit")
		time.Sleep(time.Second * 1)

		// Close both mock connections.
		finishChan <- true
		finishChan <- true

		// Wait for the proxy to observe the closed connections.
		t.Log("sleeping for a bit")
		time.Sleep(time.Second * 1)

		// Cancel the context.
		t.Log("cancelling context")
		cancel()

		// Wait for all goroutines to finish.
		t.Log("waiting for goroutines")
		wg.Wait()
	})

}

// TestProxyInboundConnection verifies MLAT proxy connections, transfers,
// retries, cancellation, and cleanup.
func TestProxyInboundConnection(t *testing.T) {

	testData := []byte("Test MLAT data! 1234567890")

	// Replace the remote connector for testing.
	connectToPlaneWatchOriginal := connectToPlaneWatch
	t.Cleanup(func() {
		connectToPlaneWatch = connectToPlaneWatchOriginal
	})
	connectToPlaneWatch = func(name, addr, sni string, insecure bool) (c net.Conn, err error) {
		return net.DialTimeout("tcp4", addr, time.Second*10)
	}

	// Reduce test timing intervals.
	logStatsIntervalOriginal := logStatsInterval
	errSleepTimeOriginal := errSleepTime
	t.Cleanup(func() {
		logStatsInterval = logStatsIntervalOriginal
		errSleepTime = errSleepTimeOriginal
	})
	logStatsInterval = time.Second * 1
	errSleepTime = time.Second * 1

	t.Run("could not accept connection", func(t *testing.T) {
		var err error

		ctx, cancel := context.WithCancel(context.Background())

		wg := sync.WaitGroup{}

		// Create the mock plane.watch listener.
		nl, err := nettest.NewLocalListener("tcp4")
		require.NoError(t, err)
		defer func() {
			_ = nl.Close()
		}()

		// Create the mock MLAT provider listener.
		mp, err := nettest.NewLocalListener("tcp4")
		require.NoError(t, err)
		defer func() {
			_ = mp.Close()
		}()

		// Close the MLAT listener to induce an accept error.
		_ = mp.Close()

		// Start the proxy.
		wg.Go(func() {
			ProxyMLATConnection(ctx, "MLAT", mp, nl.Addr().String(), TestClientAPIKey.String(), false, nil)
		})

		// Wait for connection attempts.
		time.Sleep(time.Second * 1)

		// Cancel the context.
		cancel()

		// Wait for all goroutines to finish.
		wg.Wait()

	})

	t.Run("could not connect to plane.watch", func(t *testing.T) {
		var err error

		ctx, cancel := context.WithCancel(context.Background())

		wg := sync.WaitGroup{}

		stopChan := make(chan bool)

		// Create the mock plane.watch listener.
		nl, err := nettest.NewLocalListener("tcp4")
		require.NoError(t, err)
		defer func() {
			_ = nl.Close()
		}()

		// Close the plane.watch listener to induce an error.
		_ = nl.Close()

		// Create the MLAT listener.
		mp, err := nettest.NewLocalListener("tcp4")
		require.NoError(t, err)
		defer func() {
			_ = mp.Close()
		}()

		// Start the proxy.
		wg.Go(func() {
			ProxyMLATConnection(ctx, "MLAT", mp, nl.Addr().String(), TestClientAPIKey.String(), false, nil)
		})

		// Start the mock mlat-client.
		wg.Go(func() {
			// Connect to the proxy.
			_, _ = net.Dial("tcp4", mp.Addr().String())

			// Wait for the test to finish.
			<-stopChan
		})

		// Wait for connection attempts.
		time.Sleep(time.Second * 1)

		stopChan <- true

		// Cancel the context.
		cancel()

		// Wait for all goroutines to finish.
		wg.Wait()
	})

	t.Run("working with context cancel", func(t *testing.T) {
		var err error

		ctx, cancel := context.WithCancel(context.Background())

		wg := sync.WaitGroup{}

		finishChan := make(chan bool)

		// Create the mock plane.watch listener.
		nl, err := nettest.NewLocalListener("tcp4")
		require.NoError(t, err)
		defer func() {
			_ = nl.Close()
		}()

		// Start a mock plane.watch echo server for one connection.
		wg.Go(func() {
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
		})

		// Create the MLAT listener.
		mp, err := nettest.NewLocalListener("tcp4")
		require.NoError(t, err)
		defer func() {
			_ = mp.Close()
		}()

		// Start the proxy.
		wg.Go(func() {
			ProxyMLATConnection(ctx, "MLAT", mp, nl.Addr().String(), TestClientAPIKey.String(), false, nil)
		})

		// Start the mock mlat-client.
		wg.Go(func() {
			buf := make([]byte, 1000)

			// Connect to the proxy.
			c, err := net.Dial("tcp4", mp.Addr().String())
			require.NoError(t, err)

			// Write data to the proxy.
			n, err := c.Write(testData)
			require.NoError(t, err, "mock mlat-client writing to connection")
			assert.Equal(t, len(testData), n)

			// Read the echoed data.
			n, err = c.Read(buf)
			require.NoError(t, err, "mock mlat-client reading from connection")
			assert.Equal(t, len(testData), n)
			assert.Equal(t, testData, buf[:n])

			// Close the connection.
			err = c.Close()
			require.NoError(t, err, "mock mlat-client closing connection")

			finishChan <- true
		})

		// Wait for the connection attempt.
		time.Sleep(time.Second * 1)

		// Wait for both data transfers.
		<-finishChan
		<-finishChan

		// Cancel the context.
		cancel()

		// Wait for all goroutines to finish.
		wg.Wait()
	})

	t.Run("working with full loop", func(t *testing.T) {
		var err error

		ctx, cancel := context.WithCancel(context.Background())

		wg := sync.WaitGroup{}

		finishChan := make(chan bool)

		// Create the mock plane.watch listener.
		nl, err := nettest.NewLocalListener("tcp4")
		require.NoError(t, err)
		defer func() {
			_ = nl.Close()
		}()

		// Start a mock plane.watch echo server for repeated connections.
		wg.Go(func() {
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
		})

		// Create the MLAT listener.
		mp, err := nettest.NewLocalListener("tcp4")
		require.NoError(t, err)
		defer func() {
			_ = mp.Close()
		}()

		// Start the proxy.
		wg.Go(func() {
			ProxyMLATConnection(ctx, "MLAT", mp, nl.Addr().String(), TestClientAPIKey.String(), false, nil)
		})

		// Start the mock mlat-client.
		wg.Go(func() {
			buf := make([]byte, 1000)

			for i := 0; i <= 2; i++ {

				// Connect to the proxy.
				t.Log("mock mlat-client attempting connection")
				c, err := net.Dial("tcp4", mp.Addr().String())
				require.NoError(t, err)

				// Write data to the proxy.
				t.Log("mock mlat-client writing data")
				n, err := c.Write(testData)
				require.NoError(t, err, "mock mlat-client writing to connection")
				assert.Equal(t, len(testData), n)

				// Read the echoed data.
				t.Log("mock mlat-client reading data")
				n, err = c.Read(buf)
				require.NoError(t, err, "mock mlat-client reading from connection")
				assert.Equal(t, len(testData), n)
				assert.Equal(t, testData, buf[:n])

				// Close the connection.
				t.Log("mock mlat-client closing connection")
				err = c.Close()
				require.NoError(t, err, "mock mlat-client closing connection")

			}

			finishChan <- true
		})

		// Wait for both sides to complete all transfers.
		<-finishChan
		<-finishChan

		// Close the remote server.
		_ = nl.Close()

		// Cancel the context.
		cancel()

		// Wait for all goroutines to finish.
		wg.Wait()

	})
}
