package stunnel

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

func StunnelConnect(name, addr, sni string) (c *tls.Conn, err error) {

	log := log.With().Str("name", name).Str("addr", addr).Logger()

	// split host/port from addr
	remoteHost := strings.Split(addr, ":")[0]

	// define custom cert verification function
	customVerify := func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {

		// for each cert in chain sent by server
		for _, rawCert := range rawCerts {

			// parse the cert
			cert, err := x509.ParseCertificate(rawCert)
			if err != nil {
				log.Err(err).Msg("could not parse server cert")
				return err
			}

			// if the certificate is not a CA, then check it
			if !cert.IsCA {

				// ensure the certificate hostname matches the host we're trying to connect to
				err := cert.VerifyHostname(remoteHost)
				if err != nil {
					log.Err(err).Str("host", remoteHost).Msg("could not verify server cert hostname")
					return err
				}

				// load system cert pool CAs
				scp, err := x509.SystemCertPool()
				if err != nil {
					log.Err(err).Caller().Msg("could not use system cert pool")
					return err
				}
				err = addEmbeddedCertsToCertPool(scp)
				if err != nil {
					return err
				}

				// TODO: fix this
				// verify server cert
				vo := x509.VerifyOptions{}
				vo.Roots = scp
				vo.Intermediates = scp
				vo.DNSName = remoteHost
				_, err = cert.Verify(vo)
				if err != nil {
					return err
				}
			}
		}
		return nil
	}

	// load root CAs
	scp, err := x509.SystemCertPool()
	if err != nil {
		// log.Err(err).Caller().Msg("could not use system cert pool")
		return c, err
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
		// log.Err(err).Caller().Msg("could not connect")
		return c, err
	}

	// perform handshake
	err = c.Handshake()
	if err != nil {
		// log.Err(err).Caller().Msg("handshake error")
		return c, err
	}

	// log.Debug().Msg("endpoint connected")
	return c, err

}
