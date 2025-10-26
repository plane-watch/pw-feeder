package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func connHandlerEcho(t *testing.T, conn net.Conn) {
	// handles incoming connections
	// echoes all data back to client

	var (
		sendRecvBufferSize = 256 * 1024 // 256kB
	)

	defer func() {
		_ = conn.Close()
	}()

	buf := make([]byte, sendRecvBufferSize)
	for {

		// read data
		bytesRead, err := conn.Read(buf)
		if err != nil {
			break
		}

		// echo data back
		_, err = conn.Write(buf[:bytesRead])
		if err != nil {
			break
		}
	}
}

func connHandlerChan(t *testing.T, conn net.Conn, dataIn, dataOut chan []byte) {
	// handles incoming connections
	// echoes all data back to client

	var (
		sendRecvBufferSize = 256 * 1024 // 256kB
	)

	defer func() {
		_ = conn.Close()
	}()

	bufOut := make([]byte, sendRecvBufferSize)

	for {

		bufIn := <-dataIn

		// write data from chan
		_, err := conn.Write(bufIn)
		if err != nil {
			t.Error(err)
		}

		// read data into chan
		bytesRead, err := conn.Read(bufOut)
		if err != nil {
			t.Error(err)
		}
		dataOut <- bufOut[:bytesRead]

	}
}

func prepCertsForTesting(t *testing.T) (certPEM, certPrivKeyPEM, caPEM *bytes.Buffer, err error) {
	// creates a CA cert and server cert (& priv keys) for testing

	// prep CA cert for testing
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

	// prep CA cert priv key for testing
	caPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		t.Log("Error creating CA private key for testing")
		t.Error(err)
		return certPEM, certPrivKeyPEM, caPEM, err
	}

	// create CA cert
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		t.Log("Error creating CA certificate for testing")
		t.Error(err)
		return certPEM, certPrivKeyPEM, caPEM, err
	}

	// PEM Encode CA cert
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

	// prep server cert
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

	// prep server priv key
	certPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		t.Log("Error creating private key for testing")
		t.Error(err)
		return certPEM, certPrivKeyPEM, caPEM, err
	}

	// self-sign the cert
	certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, &certPrivKey.PublicKey, caPrivKey)
	if err != nil {
		t.Log("Error creating server certificate for testing")
		t.Error(err)
		return certPEM, certPrivKeyPEM, caPEM, err
	}

	// PEM Encode server cert
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

func startTLSServer(t *testing.T, wg *sync.WaitGroup, addr string) {
	// a simple TLS echo server for testing

	// prep certs for test
	t.Log("creating self-signed cert for testing")
	certPEM, certPrivKeyPEM, caPEM, err := prepCertsForTesting(t)
	if err != nil {
		t.Log("Error creating self-signed cert for testing")
		t.Error(err)
	}
	assert.NoError(t, err)

	// prep x509 keypair for test
	t.Log("creating self-signed x509 keypair for testing")
	serverCert, err := tls.X509KeyPair(certPEM.Bytes(), certPrivKeyPEM.Bytes())
	if err != nil {
		t.Log("Error creating self-signed x509 keypair for testing")
		t.Error(err)
	}
	assert.NoError(t, err)

	// prep cert pool
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(caPEM.Bytes())

	// tls config for testing server
	tlsConfig := tls.Config{}
	tlsConfig.Certificates = []tls.Certificate{serverCert}

	// start TLS server
	t.Log("start tls.Listen")
	tlsListener, err := tls.Listen("tcp", addr, &tlsConfig)
	if err != nil {
		t.Error(err)
	}
	defer func() {
		_ = tlsListener.Close()
	}()

	wg.Done()

	// handle incoming connections
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

func startTCPServer(t *testing.T, wg *sync.WaitGroup, addr string, dataIn, dataOut chan []byte) {
	// a simple TCP server for testing
	// should connect to an echo server
	// data can be sent through "the system" via dataIn chan
	// data should end up in the dataOut chan

	// start server
	t.Log("start net.Listen")
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		t.Error(err)
	}
	defer func() {
		_ = listener.Close()
	}()

	wg.Done()

	// handle incoming connections
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

func startTCPClient(t *testing.T, addr string, dataIn, dataOut chan []byte) {
	// a simple TCP client for testing

	var (
		sendRecvBufferSize = 256 * 1024 // 256kB
	)

	buf := make([]byte, sendRecvBufferSize)

	// start client
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
