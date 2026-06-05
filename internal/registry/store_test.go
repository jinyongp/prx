package registry

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestStoreReadMissingReturnsEmpty(t *testing.T) {
	s := Open(filepath.Join(t.TempDir(), "registry.json"))
	reg, err := s.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(reg.Services) != 0 || reg.Version != SchemaVersion {
		t.Fatalf("unexpected empty registry: %+v", reg)
	}
}

func TestStoreUpdateRoundTripAndPerm(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.json")
	s := Open(path)
	err := s.Update(func(r *Registry) error {
		return r.Reserve(Reservation{Project: "a", Service: "web", Domain: "x.localhost", Port: 4300})
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("perm = %o, want 600", perm)
	}
	reg, _ := s.Read()
	if got, ok := reg.Get("a/web"); !ok || got.Port != 4300 {
		t.Fatalf("roundtrip failed: %+v", reg)
	}
}

func TestStoreRejectsFutureSchemaWithoutRewriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.json")
	original := []byte(`{
  "version": 999,
  "services": {
    "/web": {
      "service": "web",
      "domain": "web.localhost",
      "port": 4312
    }
  }
}
`)
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	store := Open(path)
	if _, err := store.Read(); !isUnsupportedSchema(err) {
		t.Fatalf("Read error = %v, want UnsupportedSchemaError", err)
	}
	if err := store.Update(func(r *Registry) error {
		return r.Reserve(Reservation{Service: "api", Domain: "api.localhost", Port: 4313})
	}); !isUnsupportedSchema(err) {
		t.Fatalf("Update error = %v, want UnsupportedSchemaError", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatalf("future registry was rewritten:\n%s", got)
	}
}

func TestStoreRejectsMalformedReservations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.json")
	original := []byte(`{
  "version": 2,
  "services": {
    "wrong/key": {
      "project": "demo",
      "service": "web",
      "domain": "web.localhost",
      "port": 4312
    },
    "demo/api": {
      "project": "demo",
      "service": "api",
      "domain": "web.localhost",
      "port": 4313
    }
  }
}
`)
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	store := Open(path)
	_, err := store.Read()
	var integrity *IntegrityError
	if !errors.As(err, &integrity) {
		t.Fatalf("Read error = %v, want IntegrityError", err)
	}
	if len(integrity.Issues) == 0 {
		t.Fatal("missing integrity issues")
	}
	called := false
	err = store.Update(func(*Registry) error {
		called = true
		return nil
	})
	if !errors.As(err, &integrity) {
		t.Fatalf("Update error = %v, want IntegrityError", err)
	}
	if called {
		t.Fatal("Update mutator called for malformed registry")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatalf("malformed registry was rewritten:\n%s", got)
	}
}

// TestStoreConcurrentReserve verifies the flock serialises read-modify-write:
// N goroutines each reserve a distinct key/port and none is lost.
func TestStoreConcurrentReserve(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.json")
	s := Open(path)
	const n = 50

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			err := s.Update(func(r *Registry) error {
				return r.Reserve(Reservation{
					Project: "p",
					Service: fmt.Sprintf("s%d", i),
					Domain:  fmt.Sprintf("s%d.localhost", i),
					Port:    4300 + i,
				})
			})
			if err != nil {
				t.Errorf("Update %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	reg, _ := s.Read()
	if len(reg.Services) != n {
		t.Fatalf("got %d reservations, want %d (lost update)", len(reg.Services), n)
	}
}

func isUnsupportedSchema(err error) bool {
	var unsupported *UnsupportedSchemaError
	return errors.As(err, &unsupported)
}
