package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

func stunnelConnect(name, addr, sni string) (c *tls.Conn, err error) {

	logger := log.With().Str("name", name).Str("addr", addr).Logger()

	// split host/port from addr
	remoteHost := strings.Split(addr, ":")[0]

	// load root CAs
	scp, err := x509.SystemCertPool()
	if err != nil {
		logger.Err(err).Caller().Msg("could not use system cert pool")
		return c, err
	}

	// define custom cert verification function
	customVerify := func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {

		// for each cert in chain sent by server
		for _, rawCert := range rawCerts {

			// parse the cert
			cert, err := x509.ParseCertificate(rawCert)
			if err != nil {
				logger.Err(err).Msg("could not parse server cert")
				return err
			}

			// if the certificate is not a CA, then check it
			if !cert.IsCA {

				// ensure the certificate hostname matches the host we're trying to connect to
				err := cert.VerifyHostname(remoteHost)
				if err != nil {
					logger.Err(err).Str("host", remoteHost).Msg("could not verify server cert hostname")
					return err
				}

				// TODO: fix this
				// verify server cert
				vo := x509.VerifyOptions{}
				vo.Roots = scp
				vo.Intermediates = scp
				_, err = cert.Verify(vo)
				if err != nil {
					logger.Warn().AnErr("err", err).Caller().Msg("could not verify server cert")
					fmt.Println(err)
					// return err
				}

				// check validity dates
				if time.Now().Before(cert.NotBefore) {
					logger.Err(err).Caller().Time("notbefore", cert.NotBefore).Msg("time.Now() is before cert notbefore")
					return errors.New("time.Now() is before cert notbefore")
				}
				if time.Now().After(cert.NotAfter) {
					logger.Err(err).Caller().Time("notafter", cert.NotAfter).Msg("time.Now() is after cert notafter")
					return errors.New("time.Now() is before cert notafter")
				}
			}
		}
		return nil
	}

	// set up tls config
	tlsConfig := tls.Config{
		RootCAs:               scp,
		ServerName:            sni,
		InsecureSkipVerify:    true,
		VerifyPeerCertificate: customVerify,
	}

	d := net.Dialer{
		Timeout: 10 * time.Second,
	}

	// dial remote
	c, err = tls.DialWithDialer(&d, "tcp", addr, &tlsConfig)
	if err != nil {
		logger.Err(err).Caller().Msg("could not connect")
		return c, err
	}

	// perform handshake
	err = c.Handshake()
	if err != nil {
		logger.Err(err).Caller().Msg("handshake error")
		return c, err
	}

	logger.Debug().Msg("endpoint connected")
	return c, err

}
