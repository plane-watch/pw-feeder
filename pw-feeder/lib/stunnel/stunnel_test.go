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
	testSNI  = uuid.New()
	testData = []byte("the quick brown fox jumps over the lazy dog 9876543210 times")
)

func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.UnixDate})
}

func GenerateSelfSignedTLSCertAndKey(keyFile, certFile *os.File) error {
	// Thanks to: https://go.dev/src/crypto/tls/generate_cert.go

	// prep certificate info
	hosts := []string{"localhost"}
	ipAddrs := []net.IP{net.IPv4(127, 0, 0, 1)}
	notBefore := time.Now()
	notAfter := time.Now().Add(time.Minute * 15)
	isCA := true

	// generate private key
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}

	keyUsage := x509.KeyUsageDigitalSignature

	// generate serial number
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return err
	}

	// prep cert template
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

	// add hostname(s)
	for _, host := range hosts {
		template.DNSNames = append(template.DNSNames, host)
	}

	// add ip(s)
	for _, ip := range ipAddrs {
		template.IPAddresses = append(template.IPAddresses, ip)
	}

	// if self-signed, include CA
	if isCA {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign
	}

	// create certificate
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, priv.Public().(ed25519.PublicKey), priv)
	if err != nil {
		return err
	}

	// encode certificate
	err = pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	if err != nil {
		return err
	}

	// marhsal private key
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return err
	}

	// write private key
	err = pem.Encode(keyFile, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})
	if err != nil {
		return err
	}

	return nil
}

func TestStunnel(t *testing.T) {

	// prep cert file
	certFile, err := os.CreateTemp("", "bordercontrol_unit_testing_*_cert.pem")
	require.NoError(t, err, "prep cert file")
	t.Cleanup(func() {
		certFile.Close()
		os.Remove(certFile.Name())
	})

	// prep key file
	keyFile, err := os.CreateTemp("", "bordercontrol_unit_testing_*_key.pem")
	require.NoError(t, err, "prep key file")
	t.Cleanup(func() {
		keyFile.Close()
		os.Remove(keyFile.Name())
	})

	// generate cert/key for testing
	err = GenerateSelfSignedTLSCertAndKey(keyFile, certFile)
	require.NoError(t, err, "generate cert/key for testing")

	// prep tlsConfig for listener
	cert, err := tls.LoadX509KeyPair(certFile.Name(), keyFile.Name())
	require.NoError(t, err, "load cert & key from file")
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	// get listener addr
	listener, err := nettest.NewLocalListener("tcp4")
	require.NoError(t, err)

	// prep listener
	tlsListener := tls.NewListener(listener, tlsConfig)

	// prep test config
	testCtx, testCancel := context.WithCancel(context.Background())

	wgOuter := sync.WaitGroup{}

	// launch listener accepter
	wgOuter.Add(1)
	go func(t *testing.T) {
		defer wgOuter.Done()

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

				c.Close()
			}
		}
	}(t)

	conn, err := StunnelConnect("TEST", listener.Addr().String(), testSNI.String(), true)
	require.NoError(t, err)

	// test write
	n, err := conn.Write(testData)
	require.NoError(t, err)
	assert.Equal(t, len(testData), n)

	// test read
	buf := make([]byte, 1000)
	n, err = conn.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, len(testData), n)
	assert.Equal(t, testData, buf[:n])

	// close
	conn.Close()

	// clean up
	testCancel()
	wgOuter.Wait()

}

func TestStunnel_cantconnect(t *testing.T) {

	// get listener addr
	listener, err := nettest.NewLocalListener("tcp4")
	require.NoError(t, err)

	// introduce error
	listener.Close()

	// test
	_, err = StunnelConnect("TEST", listener.Addr().String(), testSNI.String(), true)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "connection refused"))

}

func TestStunnel_tlserror(t *testing.T) {

	// get listener addr
	listener, err := nettest.NewLocalListener("tcp4")
	require.NoError(t, err)

	// prep test config
	testCtx, testCancel := context.WithCancel(context.Background())

	wgOuter := sync.WaitGroup{}

	// launch listener accepter
	wgOuter.Add(1)
	go func(t *testing.T) {
		defer wgOuter.Done()

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

				c.Close()
			}
		}
	}(t)

	// test
	_, err = StunnelConnect("TEST", listener.Addr().String(), testSNI.String(), true)
	require.Error(t, err)

	// clean up
	testCancel()
	wgOuter.Wait()

}
