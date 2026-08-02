package stunnel

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/nettest"
)

var (
	// testSNI is the TLS server name used by connection tests.
	testSNI = uuid.New()

	// testData is the payload exchanged by test connections.
	testData = []byte("the quick brown fox jumps over the lazy dog 9876543210 times")
)

// init configures console logging for the package tests.
func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.UnixDate})
}

// GenerateSelfSignedTLSCertAndKey writes a self-signed test certificate and its
// private key to the supplied files.
func GenerateSelfSignedTLSCertAndKey(keyFile, certFile *os.File) error {
	// Based on: https://go.dev/src/crypto/tls/generate_cert.go

	// Prepare the certificate details.
	hosts := []string{"localhost"}
	ipAddrs := []net.IP{net.IPv4(127, 0, 0, 1)}
	notBefore := time.Now()
	notAfter := time.Now().Add(time.Minute * 15)
	//isCA := true

	// Generate the private key.
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}

	keyUsage := x509.KeyUsageDigitalSignature

	// Generate the serial number.
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return err
	}

	// Prepare the certificate template.
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"plane.watch"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              keyUsage,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Add the hostnames.
	for _, host := range hosts {
		template.DNSNames = append(template.DNSNames, host)
	}

	// Add the IP addresses.
	for _, ip := range ipAddrs {
		template.IPAddresses = append(template.IPAddresses, ip)
	}

	// Include the certificate authority attributes for self-signing.
	//if isCA {
	template.IsCA = true
	template.KeyUsage |= x509.KeyUsageCertSign
	//}

	// Create the certificate.
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, priv.Public().(ed25519.PublicKey), priv)
	if err != nil {
		return err
	}

	// Encode the certificate.
	err = pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	if err != nil {
		return err
	}

	// Marshal the private key.
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return err
	}

	// Write the private key.
	err = pem.Encode(keyFile, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})
	if err != nil {
		return err
	}

	return nil
}

// GenerateTLSCertificateChain creates a root, intermediate, and leaf certificate
// for verification tests.
func GenerateTLSCertificateChain(t *testing.T) (*x509.Certificate, *x509.Certificate, *x509.Certificate) {
	t.Helper()

	notBefore := time.Now().Add(-time.Minute)
	notAfter := time.Now().Add(time.Minute * 15)
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)

	newSerialNumber := func() *big.Int {
		t.Helper()

		serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
		require.NoError(t, err)

		return serialNumber
	}

	_, rootPriv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	rootTemplate := x509.Certificate{
		SerialNumber: newSerialNumber(),
		Subject: pkix.Name{
			CommonName: "test root",
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}

	rootDER, err := x509.CreateCertificate(rand.Reader, &rootTemplate, &rootTemplate, rootPriv.Public().(ed25519.PublicKey), rootPriv)
	require.NoError(t, err)

	rootCert, err := x509.ParseCertificate(rootDER)
	require.NoError(t, err)

	_, intermediatePriv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	intermediateTemplate := x509.Certificate{
		SerialNumber: newSerialNumber(),
		Subject: pkix.Name{
			CommonName: "test intermediate",
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            0,
	}

	intermediateDER, err := x509.CreateCertificate(rand.Reader, &intermediateTemplate, rootCert, intermediatePriv.Public().(ed25519.PublicKey), rootPriv)
	require.NoError(t, err)

	intermediateCert, err := x509.ParseCertificate(intermediateDER)
	require.NoError(t, err)

	_, leafPriv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	leafTemplate := x509.Certificate{
		SerialNumber: newSerialNumber(),
		Subject: pkix.Name{
			CommonName: "localhost",
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.IPv4(127, 0, 0, 1)},
	}

	leafDER, err := x509.CreateCertificate(rand.Reader, &leafTemplate, intermediateCert, leafPriv.Public().(ed25519.PublicKey), intermediatePriv)
	require.NoError(t, err)

	leafCert, err := x509.ParseCertificate(leafDER)
	require.NoError(t, err)

	return rootCert, intermediateCert, leafCert
}

// TestVerifyPeerCertificatesUsesPresentedIntermediates verifies that presented
// intermediate certificates are included in chain validation.
func TestVerifyPeerCertificatesUsesPresentedIntermediates(t *testing.T) {

	rootCert, intermediateCert, leafCert := GenerateTLSCertificateChain(t)

	roots := x509.NewCertPool()
	roots.AddCert(rootCert)

	require.NoError(t, verifyPeerCertificates([]*x509.Certificate{leafCert, intermediateCert}, roots, "localhost"))
	require.Error(t, verifyPeerCertificates([]*x509.Certificate{leafCert}, roots, "localhost"))

}

// TestGetSystemCertPoolCached verifies that verified connections share roots.
func TestGetSystemCertPoolCached(t *testing.T) {
	first, err := getSystemCertPool()
	require.NoError(t, err)

	second, err := getSystemCertPool()
	require.NoError(t, err)

	assert.Same(t, first, second)
}

// TestStunnel verifies a successful bidirectional TLS connection.
func TestStunnel(t *testing.T) {

	// Prepare the certificate file.
	certFile, err := os.CreateTemp("", "bordercontrol_unit_testing_*_cert.pem")
	require.NoError(t, err, "prep cert file")
	t.Cleanup(func() {
		_ = certFile.Close()
		_ = os.Remove(certFile.Name())
	})

	// Prepare the key file.
	keyFile, err := os.CreateTemp("", "bordercontrol_unit_testing_*_key.pem")
	require.NoError(t, err, "prep key file")
	t.Cleanup(func() {
		_ = keyFile.Close()
		_ = os.Remove(keyFile.Name())
	})

	// Generate the certificate and key.
	err = GenerateSelfSignedTLSCertAndKey(keyFile, certFile)
	require.NoError(t, err, "generate cert/key for testing")

	// Prepare the listener TLS configuration.
	cert, err := tls.LoadX509KeyPair(certFile.Name(), keyFile.Name())
	require.NoError(t, err, "load cert & key from file")
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	// Create the TCP listener.
	listener, err := nettest.NewLocalListener("tcp4")
	require.NoError(t, err)

	// Wrap the listener with TLS.
	tlsListener := tls.NewListener(listener, tlsConfig)

	// Prepare the test context.
	testCtx, testCancel := context.WithCancel(context.Background())

	wgOuter := sync.WaitGroup{}

	// Start the listener accept loop.
	wgOuter.Go(func() {

		buf := make([]byte, 1000)

		for {
			select {
			case <-testCtx.Done():
				return
			default:
				err := listener.(*net.TCPListener).SetDeadline(time.Now().Add(time.Second))
				require.NoError(t, err)

				c, err := tlsListener.Accept()
				if err != nil {
					if strings.Contains(err.Error(), "timeout") {
						continue
					} else {
						require.NoError(t, err)
					}
				}

				n, err := c.Read(buf)
				require.NoError(t, err)

				assert.True(t, c.(*tls.Conn).ConnectionState().HandshakeComplete)
				assert.Equal(t, testSNI.String(), c.(*tls.Conn).ConnectionState().ServerName)

				_, err = c.Write(buf[:n])
				require.NoError(t, err)

				_ = c.Close()
			}
		}
	})

	conn, err := Connect("TEST", listener.Addr().String(), testSNI.String(), true)
	require.NoError(t, err)

	// Test writing through the tunnel.
	n, err := conn.Write(testData)
	require.NoError(t, err)
	assert.Equal(t, len(testData), n)

	// Test reading through the tunnel.
	buf := make([]byte, 1000)
	n, err = conn.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, len(testData), n)
	assert.Equal(t, testData, buf[:n])

	// Close the client connection.
	_ = conn.Close()

	// Stop the listener loop.
	testCancel()
	wgOuter.Wait()

}

// TestStunnel_Error_CantConnect verifies errors from an unavailable endpoint.
func TestStunnel_Error_CantConnect(t *testing.T) {

	// Create a TCP listener.
	listener, err := nettest.NewLocalListener("tcp4")
	require.NoError(t, err)

	// Close the listener to induce an error.
	_ = listener.Close()

	// Attempt the connection.
	_, err = Connect("TEST", listener.Addr().String(), testSNI.String(), false)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "connection refused"))

}

// TestStunnel_Error_TLSError verifies that a non-TLS endpoint causes a TLS error.
func TestStunnel_Error_TLSError(t *testing.T) {

	// Create a TCP listener.
	listener, err := nettest.NewLocalListener("tcp4")
	require.NoError(t, err)

	// Prepare the test context.
	testCtx, testCancel := context.WithCancel(context.Background())

	wgOuter := sync.WaitGroup{}

	// Start the listener accept loop.
	wgOuter.Go(func() {

		buf := make([]byte, 1000)

		for {
			select {
			case <-testCtx.Done():
				return
			default:
				err := listener.(*net.TCPListener).SetDeadline(time.Now().Add(time.Second))
				require.NoError(t, err)

				c, err := listener.Accept()
				if err != nil {
					if strings.Contains(err.Error(), "timeout") {
						continue
					} else {
						require.NoError(t, err)
					}
				}

				n, err := c.Read(buf)
				require.NoError(t, err)

				_, err = c.Write(buf[:n])
				require.NoError(t, err)

				_ = c.Close()
			}
		}
	})

	// Attempt the TLS connection.
	_, err = Connect("TEST", listener.Addr().String(), testSNI.String(), false)
	require.Error(t, err)

	// Stop the listener loop.
	testCancel()
	wgOuter.Wait()

}
