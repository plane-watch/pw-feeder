package connproxy

import (
	"context"
	"errors"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"pw-feeder/lib/network"
	"pw-feeder/lib/stunnel"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type tunnelStats struct {
	mu            sync.RWMutex
	bytesRxLocal  uint64
	bytesTxLocal  uint64
	bytesRxRemote uint64
	bytesTxRemote uint64
}

var (
	logStatsInterval = time.Minute * 5
	errSleepTime     = time.Second * 10

	// wrapper to stunnelConnect to allow overriding for testing
	connectToPlaneWatch = func(name, addr, sni string, insecure bool) (c net.Conn, err error) {
		return stunnel.Connect(name, addr, sni, insecure)
	}
)

func (ts *tunnelStats) incrementByteCounter(bytesRxLocal, bytesTxLocal, bytesRxRemote, bytesTxRemote uint64) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.bytesRxLocal += bytesRxLocal
	ts.bytesTxLocal += bytesTxLocal
	ts.bytesRxRemote += bytesRxRemote
	ts.bytesTxRemote += bytesTxRemote
}

func (ts *tunnelStats) readStats() (bytesRxLocal, bytesTxLocal, bytesRxRemote, bytesTxRemote uint64) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.bytesRxLocal, ts.bytesTxLocal, ts.bytesRxRemote, ts.bytesTxRemote
}

func dataMover(connIn net.Conn, connOut net.Conn, log zerolog.Logger) (bytesRead, bytesWritten int, err error) {
	buf := make([]byte, 256*1024) // 256kB buffer

	// set deadline
	err = connIn.SetReadDeadline(time.Now().Add(time.Second))
	if err != nil {
		return
	}

	// attempt read
	bytesRead, err = connIn.Read(buf)
	if err != nil {

		// don't raise an error if deadline exceeded
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

func dataMoverNettoTLS(ctx context.Context, connA net.Conn, connB net.Conn, ts *tunnelStats, log zerolog.Logger) {
	log = log.With().Str("conn", "client-side").Logger()
	for {
		select {
		case <-ctx.Done():
			return
		default:
			bytesRead, bytesWritten, err := dataMover(connA, connB, log)
			if err != nil {
				return
			}
			ts.incrementByteCounter(uint64(bytesRead), 0, 0, uint64(bytesWritten))
		}
	}
}

func dataMoverTLStoNet(ctx context.Context, connA net.Conn, connB net.Conn, ts *tunnelStats, log zerolog.Logger) {
	log = log.With().Str("conn", "server-side").Logger()
	for {
		select {
		case <-ctx.Done():
			return
		default:
			bytesRead, bytesWritten, err := dataMover(connA, connB, log)
			if err != nil {
				return
			}
			ts.incrementByteCounter(0, uint64(bytesWritten), uint64(bytesRead), 0)
		}
	}
}

func logStats(ctx context.Context, ts *tunnelStats, proto string, interval time.Duration) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
			bytesRxLocal, bytesTxLocal, bytesRxRemote, bytesTxRemote := ts.readStats()
			log.Info().
				Uint64("bytesRxLocal", bytesRxLocal).
				Uint64("bytesTxLocal", bytesTxLocal).
				Uint64("bytesRxRemote", bytesRxRemote).
				Uint64("bytesTxRemote", bytesTxRemote).
				Str("proto", proto).
				Msg("connection statistics")
		}
	}
}

func ProxyBEASTConnection(ctx context.Context, protoname, localaddr, pwendpoint, apikey string, insecure bool) {

	logger := log.With().Str("src", localaddr).Str("dst", pwendpoint).Str("proto", protoname).Logger()

	outerWg := sync.WaitGroup{}

	// log stats every 5 mins
	ts := tunnelStats{}
	outerWg.Go(func() {
		logStats(ctx, &ts, protoname, logStatsInterval)
	})

	for {

		innerWg := sync.WaitGroup{}

		// if context closure
		select {
		case _ = <-ctx.Done():
			logger.Debug().Msg("stopping")
			outerWg.Wait()
			return
		default:
		}

		logger.Info().Msg("initiating connection to BEAST provider")

		// connect local end point (lc = local connection)
		lc, err := network.ConnectToHost(protoname, localaddr)
		if err != nil {
			logger.Err(err).Msg("tunnel terminated. could not connect to the local data source, please ensure it is running and listening on the specified port")
			time.Sleep(errSleepTime)
			continue
		}

		logger.Info().Msg("initiating tunnel connection to plane.watch")

		// connect plane.watch endpoint (pwc = plane.watch connection)
		pwc, err := connectToPlaneWatch(protoname, pwendpoint, apikey, insecure)
		if err != nil {
			logger.Err(err).Msg("tunnel terminated. could not connect to the plane.watch feed-in server, please check your internet connection")
			_ = lc.Close()
			time.Sleep(errSleepTime)
			continue
		}

		// update user
		logger.Info().Msg("feeding BEAST data to plane.watch")

		// start tunneling data
		// This will block until there is an error or the connection is closed

		// prep context for data movers
		dataMoverCtx, dataMoverCancel := context.WithCancel(ctx)

		innerWg.Go(func() {
			defer dataMoverCancel()
			dataMoverNettoTLS(dataMoverCtx, lc, pwc, &ts, logger)
		})

		innerWg.Go(func() {
			defer dataMoverCancel()
			dataMoverTLStoNet(dataMoverCtx, pwc, lc, &ts, logger)
		})

		// chan for waitgroup
		wgChan := make(chan bool)
		go func() {
			innerWg.Wait()
			wgChan <- true
		}()

		select {
		// if context closure, exit
		case <-ctx.Done():
			logger.Debug().Msg("stopping")
			_ = pwc.Close()
			_ = lc.Close()
			innerWg.Wait()
			outerWg.Wait()
			return

		// wait for goroutines to finish
		case <-wgChan:
			// close connections
			_ = lc.Close()
			_ = pwc.Close()
			// let user know
			logger.Warn().Msg("tunnel to plane.watch has been terminated")
		}

		// back-off
		logger.Info().Msg("reconnecting in 10 seconds")
		time.Sleep(10 * time.Second)
	}
}

func ProxyMLATConnection(ctx context.Context, protoname string, listener net.Listener, pwendpoint, apikey string, insecure bool) {

	logger := log.With().Str("listen", listener.Addr().String()).Str("dst", pwendpoint).Str("proto", protoname).Logger()
	logger.Info().Msg("listening for connections from mlat-client")

	outerWg := sync.WaitGroup{}

	// log stats every 5 mins
	ts := tunnelStats{}
	outerWg.Go(func() {
		logStats(ctx, &ts, protoname, logStatsInterval)
	})

	for {

		innerWg := sync.WaitGroup{}

		// if context closure
		select {
		case _ = <-ctx.Done():
			logger.Debug().Msg("stopping")
			outerWg.Wait()
			return
		default:
		}

		// wait for local connection with deadline
		err := listener.(*net.TCPListener).SetDeadline(time.Now().Add(time.Second * 1))
		if err != nil {
			logger.Err(err).Msg("Error setting accept deadline")
			time.Sleep(errSleepTime)
			continue
		}

		lc, err := listener.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "timeout") {
				continue
			} else {
				logger.Err(err).Msg("An error occurred attempting to accept the incoming connection")
				time.Sleep(errSleepTime)
				continue
			}
		}

		// update logger context
		logger = logger.With().Str("src", lc.RemoteAddr().String()).Logger()
		logger.Info().Msg("connection established from mlat-client")

		logger.Info().Msg("initiating tunnel connection to plane.watch")

		// connect plane.watch endpoint
		pwc, err := connectToPlaneWatch(protoname, pwendpoint, apikey, insecure)
		if err != nil {
			logger.Err(err).Msg("tunnel terminated. could not connect to the plane.watch feed-in server, please check your internet connection.")
			_ = lc.Close()
			continue
		}

		// update user
		logger.Info().Msg("feeding MLAT results to plane.watch")

		// tunnel data
		innerWg.Go(func() {
			dataMoverNettoTLS(ctx, lc, pwc, &ts, logger)
		})
		innerWg.Go(func() {
			dataMoverTLStoNet(ctx, pwc, lc, &ts, logger)
		})

		// chan for waitgroup
		wgChan := make(chan bool)
		go func() {
			innerWg.Wait()
			wgChan <- true
		}()

		select {
		// if context closure, exit
		case <-ctx.Done():
			logger.Debug().Msg("stopping")
			_ = lc.Close()
			_ = pwc.Close()
			innerWg.Wait()
			outerWg.Wait()
			return

		// wait for goroutines to finish
		case <-wgChan:
			// close connections
			_ = lc.Close()
			_ = pwc.Close()
			// let user know
			logger.Warn().Msg("tunnel to plane.watch has been terminated")
		}

		// back-off
		logger.Info().Msg("reconnecting in 10 seconds")
		time.Sleep(10 * time.Second)
	}
}
