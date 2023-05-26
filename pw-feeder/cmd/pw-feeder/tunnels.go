package main

import (
	"crypto/tls"
	"net"
	"sync"
	"time"

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

func (ts *tunnelStats) incrementByteCounter(bytesRxLocal, bytesTxLocal, bytesRxRemote, bytesTxRemote uint64) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.bytesRxLocal += ts.bytesRxLocal
	ts.bytesTxLocal += ts.bytesTxLocal
	ts.bytesRxRemote += ts.bytesRxRemote
	ts.bytesTxRemote += ts.bytesTxRemote
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

func logStats(ts *tunnelStats, logger zerolog.Logger) {
	for {
		time.Sleep(300 * time.Second)
		bytesRxLocal, bytesTxLocal, bytesRxRemote, bytesTxRemote := ts.readStats()
		logger.Info().Uint64("bytesRxLocal", bytesRxLocal).Uint64("bytesTxLocal", bytesTxLocal).Uint64("bytesRxRemote", bytesRxRemote).Uint64("bytesTxRemote", bytesTxRemote).Str("proto", "BEAST").Msg("statistics")
	}
}

func tunnelOutboundConnection(protoname, localaddr, pwendpoint, apikey string, whenDone func()) {

	logger := log.With().Str("src", localaddr).Str("dst", pwendpoint).Str("proto", protoname).Logger()

	// log stats every 5 mins
	ts := tunnelStats{}
	ts.mu = sync.RWMutex{}
	go logStats(&ts, logger)

	lastLoopTime := time.Unix(0, 0)

	for {

		// back off if looping too frequently
		if !time.Now().After(lastLoopTime.Add(30 * time.Second)) {
			logger.Debug().Msg("Sleeping for 30 seconds")
			time.Sleep(time.Second * 30)
		}
		lastLoopTime = time.Now()

		logger.Info().Msg("starting tunnel")

		// connect plane.watch endpoint
		pwc, err := stunnelConnect(protoname, pwendpoint, apikey)
		if err != nil {
			logger.Err(err).Msg("tunnel terminated. could not connect to plane watch")
			continue
		}

		// connect local end point
		lc, err := connectToHost(protoname, localaddr)
		if err != nil {
			logger.Err(err).Msg("tunnel terminated. could not connect to local host")
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
			logger.Debug().AnErr("err", err).Msg("error closing local conn")
		}
		err = pwc.Close()
		if err != nil {
			logger.Debug().AnErr("err", err).Msg("error closing remote conn")
		}

		// let user know
		logger.Warn().Msg("tunnel terminated")
	}

	whenDone()
}

func tunnelInboundConnection(protoname, localaddr, pwendpoint, apikey string, whenDone func()) {

	logger := log.With().Str("listen", localaddr).Str("dst", pwendpoint).Str("proto", protoname).Logger()

	// log stats every 5 mins
	ts := tunnelStats{}
	go logStats(&ts, logger)

	lastLoopTime := time.Unix(0, 0)

	for {

		// back off if looping too frequently
		if !time.Now().After(lastLoopTime.Add(1 * time.Second)) {
			logger.Debug().Msg("Sleeping for 1 seconds")
			time.Sleep(time.Second * 1)
		}
		lastLoopTime = time.Now()

		logger.Info().Msg("listening for incoming connections")

		// set up local listener
		ll, err := net.Listen("tcp", localaddr)
		if err != nil {
			logger.Err(err).Msg("tunnel terminated. could not set up local listener")
			continue
		}

		// wait for local connection
		lc, err := ll.Accept()
		if err != nil {
			logger.Err(err).Msg("tunnel terminated. could not accept local connection")
			continue
		}

		// update logger context
		logger := log.With().Str("listen", localaddr).Str("dst", pwendpoint).Str("proto", protoname).Str("src", lc.LocalAddr().String()).Logger()
		logger.Info().Msg("connection established")

		// connect plane.watch endpoint
		pwc, err := stunnelConnect(protoname, pwendpoint, apikey)
		if err != nil {
			logger.Err(err).Msg("tunnel terminated. could not connect to plane watch")
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
			logger.Debug().AnErr("err", err).Msg("error closing local conn")
		}
		err = ll.Close()
		if err != nil {
			logger.Debug().AnErr("err", err).Msg("error closing local listener")
		}
		err = pwc.Close()
		if err != nil {
			logger.Debug().AnErr("err", err).Msg("error closing remote conn")
		}

		// let user know
		logger.Warn().Msg("tunnel terminated")
	}

	whenDone()
}
