package ca

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func loadCA(t *testing.T) (*CA, string) {
	t.Helper()
	base := t.TempDir()
	ca, err := Load(base)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return ca, base
}

func TestLoadGeneratesRoot(t *testing.T) {
	ca, base := loadCA(t)
	if !ca.Certificate().IsCA {
		t.Fatal("root is not a CA")
	}
	if ca.Certificate().Subject.CommonName != rootCN {
		t.Fatalf("CN = %q", ca.Certificate().Subject.CommonName)
	}
	keyPath := filepath.Join(base, "ca", "root.key")
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("key perm = %o, want 600", perm)
	}
}

func TestLoadIsIdempotent(t *testing.T) {
	base := t.TempDir()
	a, err := Load(base)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Load(base)
	if err != nil {
		t.Fatal(err)
	}
	if a.Fingerprint() != b.Fingerprint() {
		t.Fatal("reloading produced a different root CA")
	}
}

func TestLoadExistingDoesNotGenerateRoot(t *testing.T) {
	base := t.TempDir()
	if _, err := LoadExisting(base); !errors.Is(err, ErrNotFound) {
		t.Fatalf("LoadExisting error = %v, want ErrNotFound", err)
	}
	if _, err := os.Stat(filepath.Join(base, "ca", "root.crt")); !os.IsNotExist(err) {
		t.Fatalf("LoadExisting generated root cert or stat failed: %v", err)
	}
}

func TestLoadExistingLoadsPersistedRoot(t *testing.T) {
	base := t.TempDir()
	generated, err := Load(base)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadExisting(base)
	if err != nil {
		t.Fatalf("LoadExisting: %v", err)
	}
	if loaded.Fingerprint() != generated.Fingerprint() {
		t.Fatal("LoadExisting loaded different root CA")
	}
}

func TestLoadCertificateDoesNotRequireRootKey(t *testing.T) {
	base := t.TempDir()
	generated, err := Load(base)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(base, "ca", "root.key")); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadCertificate(base)
	if err != nil {
		t.Fatalf("LoadCertificate: %v", err)
	}
	if loaded.Fingerprint() != generated.Fingerprint() {
		t.Fatal("LoadCertificate loaded different root CA")
	}
}

func TestLoadRepairsPermissiveRootKey(t *testing.T) {
	base := t.TempDir()
	if _, err := Load(base); err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(base, "ca", "root.key")
	if err := os.Chmod(keyPath, 0o644); err != nil { //nolint:gosec // test intentionally makes the key too broad.
		t.Fatal(err)
	}
	if _, err := Load(base); err != nil {
		t.Fatalf("Load after chmod: %v", err)
	}
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("key perm = %o, want 600", perm)
	}
}

func TestGetCertificateIssuesAndChains(t *testing.T) {
	ca, _ := loadCA(t)
	const domain = "app.example.localhost"
	cert, err := ca.GetCertificate(&tls.ClientHelloInfo{ServerName: domain})
	if err != nil {
		t.Fatalf("GetCertificate: %v", err)
	}
	if cert.Leaf.Subject.CommonName != domain {
		t.Fatalf("leaf CN = %q", cert.Leaf.Subject.CommonName)
	}

	roots := x509.NewCertPool()
	roots.AddCert(ca.Certificate())
	if _, err := cert.Leaf.Verify(x509.VerifyOptions{DNSName: domain, Roots: roots}); err != nil {
		t.Fatalf("leaf does not chain to root: %v", err)
	}
}

func TestGetCertificateCaches(t *testing.T) {
	ca, _ := loadCA(t)
	c1, _ := ca.GetCertificate(&tls.ClientHelloInfo{ServerName: "x.localhost"})
	c2, _ := ca.GetCertificate(&tls.ClientHelloInfo{ServerName: "x.localhost"})
	if c1 != c2 {
		t.Fatal("expected cached leaf to be reused")
	}
}

func TestGetCertificateRejectsEmptySNI(t *testing.T) {
	ca, _ := loadCA(t)
	if _, err := ca.GetCertificate(&tls.ClientHelloInfo{}); err == nil {
		t.Fatal("expected error for empty SNI")
	}
}

func TestGetCertificateConcurrent(t *testing.T) {
	ca, _ := loadCA(t)
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := "x.localhost"
			if i%2 == 0 {
				name = "y.localhost"
			}
			if _, err := ca.GetCertificate(&tls.ClientHelloInfo{ServerName: name}); err != nil {
				t.Errorf("GetCertificate: %v", err)
			}
		}(i)
	}
	wg.Wait()
}

func TestExportWritesCertAndFingerprint(t *testing.T) {
	ca, _ := loadCA(t)
	out := filepath.Join(t.TempDir(), "gate-root.crt")
	fp, err := ca.Export(out)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if fp != ca.Fingerprint() {
		t.Fatalf("fingerprint mismatch")
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	blk, _ := pem.Decode(b)
	if blk == nil {
		t.Fatal("exported file is not PEM")
	}
}
