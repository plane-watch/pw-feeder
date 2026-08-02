package connproxy

import (
	"context"
	"errors"
	"net"
	"os"
	"pw-feeder/lib/backoff"
	"strings"
	"sync"
	"time"

	"pw-feeder/lib/network"
	"pw-feeder/lib/stunnel"

	"github.com/dustin/go-humanize"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// tunnelStats tracks the number of bytes transferred at each end of a tunnel.
type tunnelStats struct {
	// mu protects the byte counters.
	mu sync.RWMutex
	// bytesRxLocal counts bytes read from the local connection.
	bytesRxLocal uint64
	// bytesTxLocal counts bytes written to the local connection.
	bytesTxLocal uint64
	// bytesRxRemote counts bytes read from the remote connection.
	bytesRxRemote uint64
	// bytesTxRemote counts bytes written to the remote connection.
	bytesTxRemote uint64
}

var (
	// logStatsInterval controls how often tunnel statistics are logged.
	logStatsInterval = time.Minute * 5

	// errSleepTime retains the legacy connection-error delay used by package tests.
	errSleepTime = time.Second * 10

	// connectToPlaneWatch wraps stunnel.Connect so tests can replace it.
	connectToPlaneWatch = func(name, addr, sni string, insecure bool) (c net.Conn, err error) {
		return stunnel.Connect(name, addr, sni, insecure)
	}
)

const (
	// dataMoverBufferSize is the reusable buffer size for each tunnel direction.
	dataMoverBufferSize = 32 * 1024
)

// incrementByteCounter atomically adds values to the tunnel byte counters.
func (ts *tunnelStats) incrementByteCounter(bytesRxLocal, bytesTxLocal, bytesRxRemote, bytesTxRemote uint64) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.bytesRxLocal += bytesRxLocal
	ts.bytesTxLocal += bytesTxLocal
	ts.bytesRxRemote += bytesRxRemote
	ts.bytesTxRemote += bytesTxRemote
}

// readStats atomically returns the current tunnel byte counters.
func (ts *tunnelStats) readStats() (bytesRxLocal, bytesTxLocal, bytesRxRemote, bytesTxRemote uint64) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.bytesRxLocal, ts.bytesTxLocal, ts.bytesRxRemote, ts.bytesTxRemote
}

// dataMover copies one chunk of data from connIn to connOut using buf.
func dataMover(connIn net.Conn, connOut net.Conn, buf []byte, log zerolog.Logger) (bytesRead, bytesWritten int, err error) {
	// Set a read deadline so the caller can periodically check its context.
	err = connIn.SetReadDeadline(time.Now().Add(time.Second))
	if err != nil {
		return
	}

	// Read a chunk from the source connection.
	bytesRead, err = connIn.Read(buf)
	if err != nil {

		// Treat a read deadline as an empty read rather than an error.
		if errors.Is(err, os.ErrDeadlineExceeded) {
			return 0, 0, nil
		}

		log.Err(err).Msg("error reading from socket")
		return
	}
	bytesWritten, err = connOut.Write(buf[:bytesRead])
	if err != nil {
		if strings.Contains(err.Error(), "use of closed network connection") {
			return
		}
		log.Err(err).Msg("error writing to socket")
		return
	}
	return
}

// dataMoverNettoTLS copies data from the local connection to the TLS connection
// until the context is cancelled or a transfer fails.
func dataMoverNettoTLS(ctx context.Context, connA net.Conn, connB net.Conn, ts *tunnelStats, log zerolog.Logger) {
	log = log.With().Str("conn", "client-side").Logger()
	buf := make([]byte, dataMoverBufferSize)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			bytesRead, bytesWritten, err := dataMover(connA, connB, buf, log)
			if err != nil {
				return
			}
			ts.incrementByteCounter(uint64(bytesRead), 0, 0, uint64(bytesWritten))
		}
	}
}

// dataMoverTLStoNet copies data from the TLS connection to the local connection
// until the context is cancelled or a transfer fails.
func dataMoverTLStoNet(ctx context.Context, connA net.Conn, connB net.Conn, ts *tunnelStats, log zerolog.Logger) {
	log = log.With().Str("conn", "server-side").Logger()
	buf := make([]byte, dataMoverBufferSize)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			bytesRead, bytesWritten, err := dataMover(connA, connB, buf, log)
			if err != nil {
				return
			}
			ts.incrementByteCounter(0, uint64(bytesWritten), uint64(bytesRead), 0)
		}
	}
}

// logStats periodically logs tunnel byte counters until the context is cancelled.
func logStats(ctx context.Context, ts *tunnelStats, proto string, interval time.Duration) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
			bytesRxLocal, bytesTxLocal, bytesRxRemote, bytesTxRemote := ts.readStats()
			log.Info().
				Str("RxLocal", humanize.Bytes(bytesRxLocal)).
				Str("TxLocal", humanize.Bytes(bytesTxLocal)).
				Str("RxRemote", humanize.Bytes(bytesRxRemote)).
				Str("TxRemote", humanize.Bytes(bytesTxRemote)).
				Str("proto", proto).
				Msg("connection statistics")
		}
	}
}

// ProxyBEASTConnection continuously proxies BEAST data from a local endpoint to
// plane.watch until the context is cancelled.
func ProxyBEASTConnection(
	ctx context.Context,
	protoname, localaddr, pwendpoint, apikey string,
	insecure bool,
	reg prometheus.Registerer,
) {

	logger := log.With().Str("src", localaddr).Str("dst", pwendpoint).Str("proto", protoname).Logger()

	outerWg := sync.WaitGroup{}

	// Log tunnel statistics at the configured interval.
	ts := tunnelStats{}
	outerWg.Go(func() {
		logStats(ctx, &ts, protoname, logStatsInterval)
	})

	if reg != nil {
		// define prometheus stats collectors
		colBytesRxLocal := prometheus.NewCounterFunc(
			prometheus.CounterOpts{
				Namespace:   "plane.watch",
				Subsystem:   "BEAST",
				Name:        "bytesRxLocal",
				Help:        "Byte count for BEAST traffic received from BEASTHOST",
				Unit:        "b",
				ConstLabels: nil,
			},
			func() float64 {
				ts.mu.RLock()
				defer ts.mu.RUnlock()
				return float64(ts.bytesRxLocal)
			},
		)
		colBytesTxLocal := prometheus.NewCounterFunc(
			prometheus.CounterOpts{
				Namespace:   "plane.watch",
				Subsystem:   "BEAST",
				Name:        "bytesTxLocal",
				Help:        "Byte count for BEAST traffic sent to BEASTHOST",
				Unit:        "b",
				ConstLabels: nil,
			},
			func() float64 {
				ts.mu.RLock()
				defer ts.mu.RUnlock()
				return float64(ts.bytesTxLocal)
			},
		)
		colBytesRxRemote := prometheus.NewCounterFunc(
			prometheus.CounterOpts{
				Namespace:   "plane.watch",
				Subsystem:   "BEAST",
				Name:        "bytesRxRemote",
				Help:        "Byte count for BEAST traffic received from plane.watch",
				Unit:        "b",
				ConstLabels: nil,
			},
			func() float64 {
				ts.mu.RLock()
				defer ts.mu.RUnlock()
				return float64(ts.bytesRxRemote)
			},
		)
		colBytesTxRemote := prometheus.NewCounterFunc(
			prometheus.CounterOpts{
				Namespace:   "plane.watch",
				Subsystem:   "BEAST",
				Name:        "bytesTxRemote",
				Help:        "Byte count for BEAST traffic sent to plane.watch",
				Unit:        "b",
				ConstLabels: nil,
			},
			func() float64 {
				ts.mu.RLock()
				defer ts.mu.RUnlock()
				return float64(ts.bytesTxRemote)
			},
		)

		// register prometheus stats collectors
		err := reg.Register(colBytesRxLocal)
		if err != nil {
			logger.Error().Err(err).Str("collector", "bytesRxLocal").Msg("error registering metric")
		}
		defer reg.Unregister(colBytesRxLocal)
		err = reg.Register(colBytesTxLocal)
		if err != nil {
			logger.Error().Err(err).Str("collector", "bytesTxLocal").Msg("error registering metric")
		}
		defer reg.Unregister(colBytesTxLocal)
		err = reg.Register(colBytesRxRemote)
		if err != nil {
			logger.Error().Err(err).Str("collector", "bytesRxRemote").Msg("error registering metric")
		}
		defer reg.Unregister(colBytesRxRemote)
		err = reg.Register(colBytesTxRemote)
		if err != nil {
			logger.Error().Err(err).Str("collector", "bytesTxRemote").Msg("error registering metric")
		}
		defer reg.Unregister(colBytesTxRemote)
	}

	bo := backoff.New(backoff.WithResetAfter(5 * time.Minute))
	retry := false

	for {

		innerWg := sync.WaitGroup{}

		// Stop before starting another connection when the context is cancelled.
		select {
		case _ = <-ctx.Done():
			logger.Debug().Msg("stopping")
			outerWg.Wait()
			return
		default:
		}

		if retry {
			sleepTime := bo.BackOff()
			if sleepTime > 0 {
				logger.Info().Msgf("retrying in %s seconds", sleepTime.String())
			} else {
				logger.Info().Msg("retrying")
			}
			time.Sleep(sleepTime)
		}
		retry = true

		logger.Info().Msg("initiating connection to BEAST provider")

		// Connect to the local endpoint (lc is the local connection).
		lc, err := network.ConnectToHost(protoname, localaddr)
		if err != nil {
			logger.Err(err).Msg("tunnel terminated. could not connect to the local data source, please ensure it is running and listening on the specified port")
			continue
		}

		logger.Info().Msg("initiating tunnel connection to plane.watch")

		// Connect to plane.watch (pwc is the plane.watch connection).
		pwc, err := connectToPlaneWatch(protoname, pwendpoint, apikey, insecure)
		if err != nil {
			logger.Err(err).Msg("tunnel terminated. could not connect to the plane.watch feed-in server, please check your internet connection")
			_ = lc.Close()
			continue
		}

		// Report that the tunnel is ready.
		logger.Info().Msg("feeding BEAST data to plane.watch")

		// Start tunnelling data. The data movers stop when a transfer fails or a
		// connection is closed.

		// Prepare a shared context for the data movers.
		dataMoverCtx, dataMoverCancel := context.WithCancel(ctx)

		innerWg.Go(func() {
			defer dataMoverCancel()
			dataMoverNettoTLS(dataMoverCtx, lc, pwc, &ts, logger)
		})

		innerWg.Go(func() {
			defer dataMoverCancel()
			dataMoverTLStoNet(dataMoverCtx, pwc, lc, &ts, logger)
		})

		// Close a channel when the inner wait group finishes so the notifier can
		// never block if context cancellation wins the select.
		wgDone := make(chan struct{})
		go func() {
			innerWg.Wait()
			close(wgDone)
		}()

		select {
		// Stop when the parent context is cancelled.
		case <-ctx.Done():
			logger.Debug().Msg("stopping")
			_ = pwc.Close()
			_ = lc.Close()
			innerWg.Wait()
			outerWg.Wait()
			return

		// Clean up when the data movers finish.
		case <-wgDone:
			// Close both ends of the tunnel.
			_ = lc.Close()
			_ = pwc.Close()
			// Report the terminated tunnel.
			logger.Warn().Msg("tunnel to plane.watch has been terminated")
		}
	}
}

// ProxyMLATConnection accepts local MLAT connections and proxies their data to
// plane.watch until the context is cancelled.
func ProxyMLATConnection(
	ctx context.Context,
	protoname string,
	listener net.Listener,
	pwendpoint, apikey string,
	insecure bool,
	reg prometheus.Registerer,
) {

	logger := log.With().Str("listen", listener.Addr().String()).Str("dst", pwendpoint).Str("proto", protoname).Logger()
	logger.Info().Msg("listening for connections from mlat-client")

	outerWg := sync.WaitGroup{}

	// Log tunnel statistics at the configured interval.
	ts := tunnelStats{}
	outerWg.Go(func() {
		logStats(ctx, &ts, protoname, logStatsInterval)
	})

	if reg != nil {
		// define prometheus stats collectors
		colBytesRxLocal := prometheus.NewCounterFunc(
			prometheus.CounterOpts{
				Namespace:   "plane.watch",
				Subsystem:   "MLAT",
				Name:        "bytesRxLocal",
				Help:        "Byte count for MLAT traffic received from mlat-client",
				Unit:        "b",
				ConstLabels: nil,
			},
			func() float64 {
				ts.mu.RLock()
				defer ts.mu.RUnlock()
				return float64(ts.bytesRxLocal)
			},
		)
		colBytesTxLocal := prometheus.NewCounterFunc(
			prometheus.CounterOpts{
				Namespace:   "plane.watch",
				Subsystem:   "MLAT",
				Name:        "bytesTxLocal",
				Help:        "Byte count for MLAT traffic sent to mlat-client",
				Unit:        "b",
				ConstLabels: nil,
			},
			func() float64 {
				ts.mu.RLock()
				defer ts.mu.RUnlock()
				return float64(ts.bytesTxLocal)
			},
		)
		colBytesRxRemote := prometheus.NewCounterFunc(
			prometheus.CounterOpts{
				Namespace:   "plane.watch",
				Subsystem:   "MLAT",
				Name:        "bytesRxRemote",
				Help:        "Byte count for MLAT traffic received from plane.watch",
				Unit:        "b",
				ConstLabels: nil,
			},
			func() float64 {
				ts.mu.RLock()
				defer ts.mu.RUnlock()
				return float64(ts.bytesRxRemote)
			},
		)
		colBytesTxRemote := prometheus.NewCounterFunc(
			prometheus.CounterOpts{
				Namespace:   "plane.watch",
				Subsystem:   "MLAT",
				Name:        "bytesTxRemote",
				Help:        "Byte count for MLAT traffic sent to plane.watch",
				Unit:        "b",
				ConstLabels: nil,
			},
			func() float64 {
				ts.mu.RLock()
				defer ts.mu.RUnlock()
				return float64(ts.bytesTxRemote)
			},
		)

		// register prometheus stats collectors
		err := reg.Register(colBytesRxLocal)
		if err != nil {
			logger.Error().Err(err).Str("collector", "bytesRxLocal").Msg("error registering metric")
		}
		defer reg.Unregister(colBytesRxLocal)
		err = reg.Register(colBytesTxLocal)
		if err != nil {
			logger.Error().Err(err).Str("collector", "bytesTxLocal").Msg("error registering metric")
		}
		defer reg.Unregister(colBytesTxLocal)
		err = reg.Register(colBytesRxRemote)
		if err != nil {
			logger.Error().Err(err).Str("collector", "bytesRxRemote").Msg("error registering metric")
		}
		defer reg.Unregister(colBytesRxRemote)
		err = reg.Register(colBytesTxRemote)
		if err != nil {
			logger.Error().Err(err).Str("collector", "bytesTxRemote").Msg("error registering metric")
		}
		defer reg.Unregister(colBytesTxRemote)
	}

	bo := backoff.New(backoff.WithResetAfter(5 * time.Minute))
	retry := false

	for {

		innerWg := sync.WaitGroup{}

		// Stop before accepting another connection when the context is cancelled.
		select {
		case _ = <-ctx.Done():
			logger.Debug().Msg("stopping")
			outerWg.Wait()
			return
		default:
		}

		if retry {
			sleepTime := bo.BackOff()
			if sleepTime > 0 {
				logger.Info().Msgf("retrying in %s seconds", sleepTime.String())
			} else {
				logger.Info().Msg("retrying")
			}
			time.Sleep(sleepTime)
		}
		retry = true

		// Wait for a local connection with a deadline.
		err := listener.(*net.TCPListener).SetDeadline(time.Now().Add(time.Second * 1))
		if err != nil {
			logger.Err(err).Msg("Error setting accept deadline")
			continue
		}

		lc, err := listener.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "timeout") {
				retry = false
				continue
			} else {
				logger.Err(err).Msg("An error occurred attempting to accept the incoming connection")
				continue
			}
		}

		// Add the local client address only to this connection's logger.
		// Keeping the base logger unchanged avoids retaining every previous client address.
		connectionLogger := logger.With().Str("src", lc.RemoteAddr().String()).Logger()
		connectionLogger.Info().Msg("connection established from mlat-client")

		connectionLogger.Info().Msg("initiating tunnel connection to plane.watch")

		// Connect to the plane.watch endpoint.
		pwc, err := connectToPlaneWatch(protoname, pwendpoint, apikey, insecure)
		if err != nil {
			connectionLogger.Err(err).Msg("tunnel terminated. could not connect to the plane.watch feed-in server, please check your internet connection.")
			_ = lc.Close()
			continue
		}

		// Report that the tunnel is ready.
		connectionLogger.Info().Msg("feeding MLAT results to plane.watch")

		// Give both directions a shared per-connection context. When either mover
		// exits, cancellation stops its peer and releases both connections.
		dataMoverCtx, dataMoverCancel := context.WithCancel(ctx)

		innerWg.Go(func() {
			defer dataMoverCancel()
			dataMoverNettoTLS(dataMoverCtx, lc, pwc, &ts, connectionLogger)
		})
		innerWg.Go(func() {
			defer dataMoverCancel()
			dataMoverTLStoNet(dataMoverCtx, pwc, lc, &ts, connectionLogger)
		})

		// Close a channel when the inner wait group finishes so the notifier can
		// never block if context cancellation wins the select.
		wgDone := make(chan struct{})
		go func() {
			innerWg.Wait()
			close(wgDone)
		}()

		select {
		// Stop when the parent context is cancelled.
		case <-ctx.Done():
			connectionLogger.Debug().Msg("stopping")
			_ = lc.Close()
			_ = pwc.Close()
			innerWg.Wait()
			outerWg.Wait()
			return

		// Clean up when the data movers finish.
		case <-wgDone:
			// Close both ends of the tunnel.
			_ = lc.Close()
			_ = pwc.Close()
			// Report the terminated tunnel.
			connectionLogger.Warn().Msg("tunnel to plane.watch has been terminated")
		}
	}
}
