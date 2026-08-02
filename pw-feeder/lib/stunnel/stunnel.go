// Copyright (C) 2024 Plane Watch
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This file is part of pw-feeder.
//
// pw-feeder is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// pw-feeder is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with pw-feeder. If not, see <https://www.gnu.org/licenses/>.

package stunnel

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

var (
	// systemCertPoolOnce ensures the system roots are loaded only once.
	systemCertPoolOnce sync.Once

	// systemCertPool is shared read-only by all verified TLS connections.
	systemCertPool *x509.CertPool

	// systemCertPoolErr retains any error encountered while loading system roots.
	systemCertPoolErr error
)

// getSystemCertPool returns a process-wide, read-only system certificate pool.
func getSystemCertPool() (*x509.CertPool, error) {
	systemCertPoolOnce.Do(func() {
		systemCertPool, systemCertPoolErr = x509.SystemCertPool()
	})

	return systemCertPool, systemCertPoolErr
}

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
		scp, err = getSystemCertPool()
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
