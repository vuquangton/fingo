// Command devcert writes a self-signed certificate for local development.
//
//	go run ./cmd/devcert -dir .local/tls
//
// It exists because a Secure session cookie needs TLS, and a developer machine
// has no certificate authority. This is the standard library's certificate
// builder — the same primitive mkcert wraps — not a hand-rolled cipher.
//
// Never use this certificate on a network you do not control: it is trusted by
// nothing, and its key is in the working directory.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "devcert:", err)
		os.Exit(1)
	}
}

func run() error {
	dir := flag.String("dir", ".local/tls", "directory for cert.pem and key.pem")
	days := flag.Int("days", 825, "validity in days")
	flag.Parse()

	if err := os.MkdirAll(*dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", *dir, err)
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("serial: %w", err)
	}

	template := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{Organization: []string{"FinGo development"}, CommonName: "localhost"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(0, 0, *days),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}

	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return fmt.Errorf("create certificate: %w", err)
	}

	certPath := filepath.Join(*dir, "cert.pem")
	keyPath := filepath.Join(*dir, "key.pem")

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", certPath, err)
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return fmt.Errorf("marshal key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", keyPath, err)
	}

	fmt.Printf("wrote %s and %s (valid %d days, localhost + 127.0.0.1)\n", certPath, keyPath, *days)
	return nil
}
