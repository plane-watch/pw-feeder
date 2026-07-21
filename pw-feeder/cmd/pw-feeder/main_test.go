package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// connHandlerEcho echoes data received from conn until the connection fails.
func connHandlerEcho(t *testing.T, conn net.Conn) {
	var (
		sendRecvBufferSize = 256 * 1024 // Use a 256 kB buffer.
	)

	defer func() {
		_ = conn.Close()
	}()

	buf := make([]byte, sendRecvBufferSize)
	for {

		// Read data from the client.
		bytesRead, err := conn.Read(buf)
		if err != nil {
			break
		}

		// Echo the data to the client.
		_, err = conn.Write(buf[:bytesRead])
		if err != nil {
			break
		}
	}
}

// connHandlerChan exchanges connection data through the supplied channels.
func connHandlerChan(t *testing.T, conn net.Conn, dataIn, dataOut chan []byte) {
	var (
		sendRecvBufferSize = 256 * 1024 // Use a 256 kB buffer.
	)

	defer func() {
		_ = conn.Close()
	}()

	bufOut := make([]byte, sendRecvBufferSize)

	for {

		bufIn := <-dataIn

		// Write data received from the input channel.
		_, err := conn.Write(bufIn)
		if err != nil {
			t.Error(err)
		}

		// Send received data to the output channel.
		bytesRead, err := conn.Read(bufOut)
		if err != nil {
			t.Error(err)
		}
		dataOut <- bufOut[:bytesRead]

	}
}

// prepCertsForTesting creates a certificate authority and a signed server
// certificate for TLS tests.
func prepCertsForTesting(t *testing.T) (certPEM, certPrivKeyPEM, caPEM *bytes.Buffer, err error) {
	// Prepare the test CA certificate.
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(42069247),
		Subject: pkix.Name{
			Organization:  []string{"plane.watch Testing Certificate Authority"},
			Country:       []string{"AU"},
			Province:      []string{"Perth"},
			Locality:      []string{"Western Australia"},
			StreetAddress: []string{"123 Testing Terrace"},
			PostalCode:    []string{"6000"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	// Generate the CA private key.
	caPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		t.Log("Error creating CA private key for testing")
		t.Error(err)
		return certPEM, certPrivKeyPEM, caPEM, err
	}

	// Create the CA certificate.
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		t.Log("Error creating CA certificate for testing")
		t.Error(err)
		return certPEM, certPrivKeyPEM, caPEM, err
	}

	// PEM-encode the CA certificate and private key.
	caPEM = new(bytes.Buffer)
	err = pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})
	if err != nil {
		t.Log("Error encoding CA certificate for testing")
		t.Error(err)
		return certPEM, certPrivKeyPEM, caPEM, err
	}
	caPrivKeyPEM := new(bytes.Buffer)
	err = pem.Encode(caPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caPrivKey),
	})
	if err != nil {
		t.Log("Error encoding CA certificate private key for testing")
		t.Error(err)
		return certPEM, certPrivKeyPEM, caPEM, err
	}

	// Prepare the server certificate.
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(1234),
		Subject: pkix.Name{
			Organization:  []string{"plane.watch Testing Certificate"},
			Country:       []string{"AU"},
			Province:      []string{"Perth"},
			Locality:      []string{"Western Australia"},
			StreetAddress: []string{"123 Testing Terrace"},
			PostalCode:    []string{"6000"},
		},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(1, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 5},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	// Generate the server private key.
	certPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		t.Log("Error creating private key for testing")
		t.Error(err)
		return certPEM, certPrivKeyPEM, caPEM, err
	}

	// Sign the server certificate with the test CA.
	certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, &certPrivKey.PublicKey, caPrivKey)
	if err != nil {
		t.Log("Error creating server certificate for testing")
		t.Error(err)
		return certPEM, certPrivKeyPEM, caPEM, err
	}

	// PEM-encode the server certificate and private key.
	certPEM = new(bytes.Buffer)
	err = pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})
	if err != nil {
		t.Log("Error encoding server certificate for testing")
		t.Error(err)
		return certPEM, certPrivKeyPEM, caPEM, err
	}
	certPrivKeyPEM = new(bytes.Buffer)
	err = pem.Encode(certPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(certPrivKey),
	})
	if err != nil {
		t.Log("Error encoding server certificate private key for testing")
		t.Error(err)
		return certPEM, certPrivKeyPEM, caPEM, err
	}

	return certPEM, certPrivKeyPEM, caPEM, err
}

// startTLSServer starts a TLS echo server for tests.
func startTLSServer(t *testing.T, wg *sync.WaitGroup, addr string) {
	// Prepare certificates for the test server.
	t.Log("creating self-signed cert for testing")
	certPEM, certPrivKeyPEM, caPEM, err := prepCertsForTesting(t)
	if err != nil {
		t.Log("Error creating self-signed cert for testing")
		t.Error(err)
	}
	assert.NoError(t, err)

	// Prepare the X.509 key pair.
	t.Log("creating self-signed x509 keypair for testing")
	serverCert, err := tls.X509KeyPair(certPEM.Bytes(), certPrivKeyPEM.Bytes())
	if err != nil {
		t.Log("Error creating self-signed x509 keypair for testing")
		t.Error(err)
	}
	assert.NoError(t, err)

	// Prepare the certificate pool.
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(caPEM.Bytes())

	// Configure the test TLS server.
	tlsConfig := tls.Config{}
	tlsConfig.Certificates = []tls.Certificate{serverCert}

	// Start the TLS server.
	t.Log("start tls.Listen")
	tlsListener, err := tls.Listen("tcp", addr, &tlsConfig)
	if err != nil {
		t.Error(err)
	}
	defer func() {
		_ = tlsListener.Close()
	}()

	wg.Done()

	// Handle incoming connections.
	for {
		c, err := tlsListener.Accept()
		if err != nil {
			break
		}
		defer func() {
			_ = c.Close()
		}()

		go connHandlerEcho(t, c)
	}
}

// startTCPServer starts a test TCP server that exchanges data through channels.
func startTCPServer(t *testing.T, wg *sync.WaitGroup, addr string, dataIn, dataOut chan []byte) {
	// Start the server.
	t.Log("start net.Listen")
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		t.Error(err)
	}
	defer func() {
		_ = listener.Close()
	}()

	wg.Done()

	// Handle incoming connections.
	for {
		c, err := listener.Accept()
		if err != nil {
			break
		}
		defer func() {
			_ = c.Close()
		}()

		go connHandlerChan(t, c, dataIn, dataOut)
	}
}

// startTCPClient starts a test TCP client that exchanges data through channels.
func startTCPClient(t *testing.T, addr string, dataIn, dataOut chan []byte) {
	var (
		sendRecvBufferSize = 256 * 1024 // Use a 256 kB buffer.
	)

	buf := make([]byte, sendRecvBufferSize)

	// Start the client.
	t.Log("start net.Dial")
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Error(err)
	}
	defer func() {
		_ = c.Close()
	}()

	for {
		_, err := c.Write(<-dataIn)
		if err != nil {
			break
		}
		bytesRead, err := c.Read(buf)
		if err != nil {
			break
		}
		dataOut <- buf[:bytesRead]
	}
}

// TestApplicationLogsAreRedacted verifies that configured secrets are removed
// from string fields and errors without affecting safe text.
func TestApplicationLogsAreRedacted(t *testing.T) {
	var output bytes.Buffer

	apiKey := uuid.New()
	secondAPIKey := uuid.New()
	redactedText := "[API_KEY_REDACTED]"
	safeText := "safe text, no redaction"
	redactList := map[string]string{
		apiKey.String():       redactedText,
		secondAPIKey.String(): redactedText,
	}

	writer := zerolog.ConsoleWriter{
		Out:     &output,
		NoColor: true,
	}
	writer.FormatPrepare = redactFromLogs(redactList)

	previousLogger := log.Logger
	log.Logger = previousLogger.Output(writer)

	t.Cleanup(func() {
		log.Logger = previousLogger
	})

	// Check safe text.
	log.Info().Msg(safeText)
	got := output.String()
	require.Contains(t, got, safeText)
	require.NotContains(t, got, redactedText)

	output.Reset()

	// Check redacted string fields.
	log.Info().Str("api_keys", apiKey.String()+","+secondAPIKey.String()).Msg("test as string")
	got = output.String()
	require.NotContains(t, got, apiKey.String())
	require.NotContains(t, got, secondAPIKey.String())
	require.Contains(t, got, redactedText+","+redactedText)

	output.Reset()

	// Check redacted error text.
	secretError := errors.New("request failed for API keys " + apiKey.String() + "," + secondAPIKey.String())
	log.Error().Err(secretError).Msg("test as error")
	got = output.String()
	require.NotContains(t, got, apiKey.String())
	require.NotContains(t, got, secondAPIKey.String())
	require.Contains(t, got, "request failed for API keys "+redactedText+","+redactedText)
}
