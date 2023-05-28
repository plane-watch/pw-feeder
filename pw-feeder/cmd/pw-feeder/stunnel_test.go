package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"testing"
	"time"
)

// TODO: finish writing tests for this module
func prepCertsForTesting(t *testing.T) error {

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
	}

	// create CA cert
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		t.Log("Error creating CA certificate for testing")
		t.Error(err)
	}

	// PEM Encode CA cert
	caPEM := new(bytes.Buffer)
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
	}

	// self-sign the cert
	certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, &certPrivKey.PublicKey, caPrivKey)
	if err != nil {
		t.Log("Error creating server certificate for testing")
		t.Error(err)
	}

	// PEM Encode server cert
	certPEM := new(bytes.Buffer)
	pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})
	certPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(certPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(certPrivKey),
	})

	return nil
}
