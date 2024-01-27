package connproxy

import (
	"context"
	"net"
	"strings"
	"sync"
	"time"

	"pw-feeder/lib/network"
	"pw-feeder/lib/stunnel"

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
	connectToPlaneWatch = func(name string, addr string, sni string) (c net.Conn, err error) {
		return stunnel.StunnelConnect(name, addr, sni)
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

func dataMover(connIn net.Conn, connOut net.Conn) (bytesRead, bytesWritten int, err error) {
	buf := make([]byte, 256*1024) // 256kB buffer
	bytesRead, err = connIn.Read(buf)
	if err != nil {
		log.Err(err).Msg("error reading from socket")
		return
	}
	bytesWritten, err = connOut.Write(buf[:bytesRead])
	if err != nil {
		log.Err(err).Msg("error writing to socket")
		return
	}
	return
}

func dataMoverNettoTLS(ctx context.Context, connA net.Conn, connB net.Conn, ts *tunnelStats) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			bytesRead, bytesWritten, err := dataMover(connA, connB)
			if err != nil {
				return
			}
			ts.incrementByteCounter(uint64(bytesRead), 0, 0, uint64(bytesWritten))
		}
	}
}

func dataMoverTLStoNet(ctx context.Context, connA net.Conn, connB net.Conn, ts *tunnelStats) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			bytesRead, bytesWritten, err := dataMover(connA, connB)
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

func ProxyOutboundConnection(ctx context.Context, protoname, localaddr, pwendpoint, apikey string) {

	log := log.With().Str("src", localaddr).Str("dst", pwendpoint).Str("proto", protoname).Logger()

	outerWg := sync.WaitGroup{}

	// log stats every 5 mins
	ts := tunnelStats{}
	outerWg.Add(1)
	go func() {
		defer outerWg.Done()
		logStats(ctx, &ts, protoname, logStatsInterval)
	}()

	for {

		innerWg := sync.WaitGroup{}

		// if context closure
		select {
		case _ = <-ctx.Done():
			log.Debug().Msg("stopping")
			outerWg.Wait()
			return
		default:
		}

		log.Info().Msg("initiating tunnel connection to plane.watch")

		// connect plane.watch endpoint
		pwc, err := connectToPlaneWatch(protoname, pwendpoint, apikey)
		if err != nil {
			log.Err(err).Msg("tunnel terminated. could not connect to the plane.watch feed-in server, please check your internet connection")
			time.Sleep(errSleepTime)
			continue
		}

		// connect local end point
		lc, err := network.ConnectToHost(protoname, localaddr)
		if err != nil {
			log.Err(err).Msg("tunnel terminated. could not connect to the local data source, please ensure it is running and listening on the specified port")
			pwc.Close()
			time.Sleep(errSleepTime)
			continue
		}

		// update user
		log.Info().Msg("connection to plane.watch established")

		// start tunneling data
		// This will block until there is an error or the connection is closed

		innerWg.Add(1)
		go func() {
			defer innerWg.Done()
			dataMoverNettoTLS(ctx, lc, pwc, &ts)
		}()

		innerWg.Add(1)
		go func() {
			defer innerWg.Done()
			dataMoverTLStoNet(ctx, pwc, lc, &ts)
		}()

		// chan for waitgroup
		wgChan := make(chan bool)
		go func() {
			innerWg.Wait()
			wgChan <- true
		}()

		select {
		// if context closure, exit
		case <-ctx.Done():
			log.Debug().Msg("stopping")
			pwc.Close()
			lc.Close()
			innerWg.Wait()
			outerWg.Wait()
			return

		// wait for goroutines to finish
		case <-wgChan:
			// close connections
			lc.Close()
			pwc.Close()
			// let user know
			log.Warn().Msg("tunnel to plane.watch has been terminated")
		}
	}
}

func ProxyInboundConnection(ctx context.Context, protoname string, listener net.Listener, pwendpoint, apikey string) {

	log := log.With().Str("listen", listener.Addr().String()).Str("dst", pwendpoint).Str("proto", protoname).Logger()

	outerWg := sync.WaitGroup{}

	// log stats every 5 mins
	ts := tunnelStats{}
	outerWg.Add(1)
	go func() {
		defer outerWg.Done()
		logStats(ctx, &ts, protoname, logStatsInterval)
	}()

	for {

		innerWg := sync.WaitGroup{}

		// if context closure
		select {
		case _ = <-ctx.Done():
			log.Debug().Msg("stopping")
			outerWg.Wait()
			return
		default:
		}

		log.Info().Msg("starting listener service")

		// wait for local connection with deadline
		err := listener.(*net.TCPListener).SetDeadline(time.Now().Add(time.Second * 1))
		if err != nil {
			log.Err(err).Msg("Error setting accept deadline")
			time.Sleep(errSleepTime)
			continue
		}
		lc, err := listener.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "timeout") {
				continue
			} else {
				log.Err(err).Msg("An error occurred attempting to accept the incoming connection")
				time.Sleep(errSleepTime)
				continue
			}
		}

		// update logger context
		log = log.With().Str("src", lc.RemoteAddr().String()).Logger()
		log.Info().Msg("connection established to local data source")

		// connect plane.watch endpoint
		pwc, err := connectToPlaneWatch(protoname, pwendpoint, apikey)
		if err != nil {
			log.Err(err).Msg("tunnel terminated. could not connect to the plane.watch feed-in server, please check your internet connection.")
			lc.Close()
			continue
		}

		// tunnel data
		innerWg.Add(1)
		go func() {
			defer innerWg.Done()
			dataMoverNettoTLS(ctx, lc, pwc, &ts)
		}()
		innerWg.Add(1)
		go func() {
			defer innerWg.Done()
			dataMoverTLStoNet(ctx, pwc, lc, &ts)
		}()

		// chan for waitgroup
		wgChan := make(chan bool)
		go func() {
			innerWg.Wait()
			wgChan <- true
		}()

		select {
		// if context closure, exit
		case <-ctx.Done():
			log.Debug().Msg("stopping")
			lc.Close()
			pwc.Close()
			innerWg.Wait()
			outerWg.Wait()
			return

		// wait for goroutines to finish
		case <-wgChan:
			// close connections
			lc.Close()
			pwc.Close()
			// let user know
			log.Warn().Msg("tunnel to plane.watch has been terminated")
		}
	}
}
