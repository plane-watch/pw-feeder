package stunnel

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"time"

	"github.com/rs/zerolog/log"
)

// Connect establishes a TLS connection to addr using sni as the server name.
// Unless insecure mode is enabled, it verifies the certificate for addr's host.
func Connect(name, addr, sni string, insecure bool) (c *tls.Conn, err error) {

	logger := log.With().Str("name", name).Str("addr", addr).Logger()

	// Extract the remote host from addr.
	remoteHost, _, err := net.SplitHostPort(addr)
	if err != nil {
		logger.Err(err).Msg("could not split remote host/port")
		return c, err
	}

	// Load the system root certificate authorities.
	var scp *x509.CertPool
	if !insecure {
		scp, err = x509.SystemCertPool()
		if err != nil {
			// log.Err(err).Caller().Msg("could not use system cert pool")
			return c, err
		}
	}

	// Configure TLS verification.
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

	// Dial the remote endpoint.
	c, err = tls.DialWithDialer(&d, "tcp", addr, &tlsConfig)
	if err != nil {
		// log.Err(err).Caller().Msg("could not connect")
		return c, err
	}

	// Perform the TLS handshake.
	err = c.Handshake()
	if err != nil {
		// log.Err(err).Caller().Msg("handshake error")
		return c, err
	}

	// log.Debug().Msg("endpoint connected")
	return c, err

}

// verifyPeerCertificates verifies the presented certificate chain for dnsName
// using the supplied root certificate pool.
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
