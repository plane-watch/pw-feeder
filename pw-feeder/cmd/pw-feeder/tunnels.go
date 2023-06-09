package main

import (
	"crypto/tls"
	"net"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

type tunnelStats struct {
	mu            sync.RWMutex
	bytesRxLocal  uint64
	bytesTxLocal  uint64
	bytesRxRemote uint64
	bytesTxRemote uint64
}

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

func dataMoverNettoTLS(connA net.Conn, connB *tls.Conn, ts *tunnelStats, wg *sync.WaitGroup) {
	buf := make([]byte, 256*1024) // 256kB buffer
	for {
		bytesRead, err := connA.Read(buf)
		if err != nil {
			break
		}

		bytesWritten, err := connB.Write(buf[:bytesRead])
		if err != nil {
			break
		}

		ts.incrementByteCounter(uint64(bytesRead), 0, 0, uint64(bytesWritten))

		// log.Debug().Int("bytes", bytesRead).Msg("Net to TLS")
	}
	wg.Done()
}

func dataMoverTLStoNet(connA *tls.Conn, connB net.Conn, ts *tunnelStats, wg *sync.WaitGroup) {
	buf := make([]byte, 256*1024) // 256kB buffer
	for {
		bytesRead, err := connA.Read(buf)
		if err != nil {
			break
		}

		bytesWritten, err := connB.Write(buf[:bytesRead])
		if err != nil {
			break
		}

		ts.incrementByteCounter(0, uint64(bytesWritten), uint64(bytesRead), 0)

		// log.Debug().Int("bytes", bytesRead).Msg("TLS to Net")
	}
	wg.Done()
}

func logStats(ts *tunnelStats, proto string) {
	for {
		time.Sleep(300 * time.Second)
		bytesRxLocal, bytesTxLocal, bytesRxRemote, bytesTxRemote := ts.readStats()
		log.Info().Uint64("bytesRxLocal", bytesRxLocal).Uint64("bytesTxLocal", bytesTxLocal).Uint64("bytesRxRemote", bytesRxRemote).Uint64("bytesTxRemote", bytesTxRemote).Str("proto", proto).Msg("Current Connection Statistics")
	}
}

func tunnelOutboundConnection(protoname, localaddr, pwendpoint, apikey string, whenDone func()) {

	logger := log.With().Str("src", localaddr).Str("dst", pwendpoint).Str("proto", protoname).Logger()

	// log stats every 5 mins
	ts := tunnelStats{}
	go logStats(&ts, protoname)

	lastLoopTime := time.Unix(0, 0)

	for {

		// back off if looping too frequently
		if !time.Now().After(lastLoopTime.Add(30 * time.Second)) {
			logger.Debug().Msg("Sleeping for 30 seconds")
			time.Sleep(time.Second * 30)
		}
		lastLoopTime = time.Now()

		logger.Info().Msg("initiating tunnel connection to plane.watch")

		// connect plane.watch endpoint
		pwc, err := stunnelConnect(protoname, pwendpoint, apikey)
		if err != nil {
			logger.Err(err).Msg("tunnel terminated. could not connect to the plane.watch feed-in server, please check your internet connection")
			continue
		}

		// connect local end point
		lc, err := connectToHost(protoname, localaddr)
		if err != nil {
			logger.Err(err).Msg("tunnel terminated. could not connect to the local data source, please ensure it is running and listening on the specified port")
			continue
		}

		// update user
		logger.Info().Msg("connection to plane.watch established")

		// start tunneling data
		// This will block until there is an error or the connection is closed
		wg := sync.WaitGroup{}
		wg.Add(1)
		go dataMoverNettoTLS(lc, pwc, &ts, &wg)
		wg.Add(1)
		go dataMoverTLStoNet(pwc, lc, &ts, &wg)

		// wait for goroutines to finish
		wg.Wait()

		// attempt to close connections
		err = lc.Close()
		if err != nil {
			logger.Debug().AnErr("err", err).Msg("Internal Error when attempting to close the connection to the local data source")
		}
		err = pwc.Close()
		if err != nil {
			logger.Debug().AnErr("err", err).Msg("Internal Error when attempting to close the remote connection to plane.watch")
		}

		// let user know
		logger.Warn().Msg("tunnel to plane.watch has been terminated, attempting to reconnect. if this message persists, please check your that your API key is valid before trying again")
	}

	whenDone()
}

func tunnelInboundConnection(protoname, localaddr, pwendpoint, apikey string, whenDone func()) {

	logger := log.With().Str("listen", localaddr).Str("dst", pwendpoint).Str("proto", protoname).Logger()

	// log stats every 5 mins
	ts := tunnelStats{}
	go logStats(&ts, protoname)

	lastLoopTime := time.Unix(0, 0)

	for {

		// back off if looping too frequently
		if !time.Now().After(lastLoopTime.Add(1 * time.Second)) {
			logger.Debug().Msg("Sleeping for 1 seconds")
			time.Sleep(time.Second * 1)
		}
		lastLoopTime = time.Now()

		logger.Info().Msg("starting listener service")

		// set up local listener
		ll, err := net.Listen("tcp", localaddr)
		if err != nil {
			logger.Err(err).Msg("Could not bind to the requested tcp listening port, please check your configuration.")
			continue
		}

		// wait for local connection
		lc, err := ll.Accept()
		if err != nil {
			logger.Err(err).Msg("An error occurred attempting to handle the incoming connection")
			continue
		}

		// update logger context
		logger := log.With().Str("listen", localaddr).Str("dst", pwendpoint).Str("proto", protoname).Str("src", lc.RemoteAddr().String()).Logger()
		logger.Info().Msg("connection established to local data source")

		// connect plane.watch endpoint
		pwc, err := stunnelConnect(protoname, pwendpoint, apikey)
		if err != nil {
			logger.Err(err).Msg("tunnel terminated. could not connect to the plane.watch feed-in server, please check your internet connection.")
			continue
		}

		// tunnel data
		wg := sync.WaitGroup{}
		wg.Add(1)
		go dataMoverNettoTLS(lc, pwc, &ts, &wg)
		wg.Add(1)
		go dataMoverTLStoNet(pwc, lc, &ts, &wg)

		// wait for goroutines to finish
		wg.Wait()

		// attempt to close connections
		err = lc.Close()
		if err != nil {
			logger.Debug().AnErr("err", err).Msg("Internal Error when attempting to close the local data source")
		}
		err = ll.Close()
		if err != nil {
			logger.Debug().AnErr("err", err).Msg("Internal Error when attempting to close the local listener")
		}
		err = pwc.Close()
		if err != nil {
			logger.Debug().AnErr("err", err).Msg("Internal Error when attempting to close the remote connection to plane.watch")
		}

		// let user know
		logger.Warn().Msg("tunnel to plane.watch has been terminated, attempting to reconnect. if this message persists, please check your that your API key is valid before trying again")
	}

	whenDone()
}
