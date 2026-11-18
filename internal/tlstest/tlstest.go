// Package tlstest provides a private CA for tests only. Production code must
// not import it; production verification uses the system roots.
package tlstest

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"
)

// CA is a throwaway certificate authority for tests.
type CA struct {
	cert *x509.Certificate
	key  crypto.Signer
}

// NewCA creates a self-signed root CA.
func NewCA(t testing.TB) *CA {
	t.Helper()

	key := newKey(t)
	tmpl := caTemplate(t, "tlstest root")
	cert := createCertificate(t, tmpl, tmpl, key.Public(), key)
	return &CA{cert: cert, key: key}
}

// Pool returns a pool that trusts only this CA.
func (ca *CA) Pool() *x509.CertPool {
	pool := x509.NewCertPool()
	pool.AddCert(ca.cert)
	return pool
}

// Issue signs a server certificate for dnsNames directly with the root.
func (ca *CA) Issue(t testing.TB, dnsNames []string, notBefore, notAfter time.Time) (certPEM, keyPEM []byte) {
	t.Helper()

	key := newKey(t)
	cert := createCertificate(t, leafTemplate(t, dnsNames, notBefore, notAfter), ca.cert, key.Public(), ca.key)
	return encodeCertificate(cert), encodeKey(t, key)
}

// IssueViaIntermediate signs a server certificate with a new intermediate CA
// that chains to the root.
func (ca *CA) IssueViaIntermediate(t testing.TB, dnsNames []string) (leafPEM, intermediatePEM, keyPEM []byte) {
	t.Helper()

	intermediateKey := newKey(t)
	intermediate := createCertificate(t, caTemplate(t, "tlstest intermediate"), ca.cert, intermediateKey.Public(), ca.key)

	key := newKey(t)
	now := time.Now()
	leaf := createCertificate(t, leafTemplate(t, dnsNames, now.Add(-time.Hour), now.Add(24*time.Hour)), intermediate, key.Public(), intermediateKey)
	return encodeCertificate(leaf), encodeCertificate(intermediate), encodeKey(t, key)
}

func newKey(t testing.TB) *ecdsa.PrivateKey {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return key
}

func serialNumber(t testing.TB) *big.Int {
	t.Helper()

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatalf("generate serial: %v", err)
	}
	return serial
}

func caTemplate(t testing.TB, commonName string) *x509.Certificate {
	t.Helper()

	now := time.Now()
	return &x509.Certificate{
		SerialNumber:          serialNumber(t),
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
}

func leafTemplate(t testing.TB, dnsNames []string, notBefore, notAfter time.Time) *x509.Certificate {
	t.Helper()

	return &x509.Certificate{
		SerialNumber: serialNumber(t),
		Subject:      pkix.Name{CommonName: "tlstest leaf"},
		DNSNames:     dnsNames,
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
}

func createCertificate(t testing.TB, tmpl, parent *x509.Certificate, pub crypto.PublicKey, signer crypto.Signer) *x509.Certificate {
	t.Helper()

	der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, pub, signer)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}
	return cert
}

func encodeCertificate(cert *x509.Certificate) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
}

func encodeKey(t testing.TB, key *ecdsa.PrivateKey) []byte {
	t.Helper()

	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}
