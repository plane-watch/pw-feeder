package stunnel

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"time"

	"github.com/rs/zerolog/log"
)

func Connect(name, addr, sni string, insecure bool) (c *tls.Conn, err error) {

	logger := log.With().Str("name", name).Str("addr", addr).Logger()

	// split host/port from addr
	remoteHost, _, err := net.SplitHostPort(addr)
	if err != nil {
		logger.Err(err).Msg("could not split remote host/port")
		return c, err
	}

	// load root CAs
	var scp *x509.CertPool
	if !insecure {
		scp, err = x509.SystemCertPool()
		if err != nil {
			// log.Err(err).Caller().Msg("could not use system cert pool")
			return c, err
		}
	}

	// set up tls config
	tlsConfig := tls.Config{
		ServerName:         sni,
		InsecureSkipVerify: true,
		VerifyConnection: func(cs tls.ConnectionState) error {
			if insecure {
				return nil
			}

			err := verifyPeerCertificates(cs.PeerCertificates, scp, remoteHost)
			if err != nil {
				logger.Err(err).Str("host", remoteHost).Msg("could not verify server certificate")
			}
			return err
		},
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

func verifyPeerCertificates(peerCertificates []*x509.Certificate, roots *x509.CertPool, dnsName string) error {

	if len(peerCertificates) == 0 {
		return errors.New("server presented no certificates")
	}

	intermediates := x509.NewCertPool()
	for _, cert := range peerCertificates[1:] {
		intermediates.AddCert(cert)
	}

	verifyOptions := x509.VerifyOptions{
		Roots:         roots,
		Intermediates: intermediates,
		DNSName:       dnsName,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	_, err := peerCertificates[0].Verify(verifyOptions)
	return err

}
