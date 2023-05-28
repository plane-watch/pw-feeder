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

func connHandler(t *testing.T, conn net.Conn) {
	// handles incoming connections

	var (
		sendRecvBufferSize = 256 * 1024 // 256kB
	)

	defer conn.Close()

	buf := make([]byte, sendRecvBufferSize)
	for {

		// read data
		_, err := conn.Read(buf)
		if err != nil {
			t.Error(err)
		}
	}
}

func prepCertsForTesting(t *testing.T) (certPEM, certPrivKeyPEM, caPEM *bytes.Buffer, err error) {

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
	pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})
	caPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(caPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caPrivKey),
	})

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
	pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})
	certPrivKeyPEM = new(bytes.Buffer)
	pem.Encode(certPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(certPrivKey),
	})

	return certPEM, certPrivKeyPEM, caPEM, err
}

func startTLSServer(t *testing.T, wg *sync.WaitGroup, addr string) {

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
	defer tlsListener.Close()

	wg.Done()

	// handle incoming connections
	for {
		c, err := tlsListener.Accept()
		if err != nil {
			t.Error(err)
		}
		defer c.Close()
		go connHandler(t, c)
	}
}

func TestStunnelConnect(t *testing.T) {

	// start test server
	t.Log("starting test SSL server")
	wg := sync.WaitGroup{}
	wg.Add(1)
	go startTLSServer(t, &wg, "127.0.0.1:32345")
	t.Log("waiting for test SSL server to start")
	wg.Wait()

	// test stunnelConnect
	t.Log("test stunnelConnect")
	c, err := stunnelConnect("test", "127.0.0.1:32345", "7BE5F7FD-BA97-4280-9B15-4F0746D875DA")
	if err != nil {
		t.Error(err)
	}
	assert.NoError(t, err)
	defer c.Close()

}
