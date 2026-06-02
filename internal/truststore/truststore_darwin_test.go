package truststore

import "testing"

func TestIsRemoveTrustedCertNotFound(t *testing.T) {
	cases := [][]byte{
		[]byte("The specified item could not be found in the keychain."),
		[]byte("unable to find certificate"),
		[]byte("certificate not found"),
	}
	for _, tc := range cases {
		if !isRemoveTrustedCertNotFound(tc) {
			t.Fatalf("not-found output not recognized: %q", string(tc))
		}
	}
	if isRemoveTrustedCertNotFound([]byte("authorization failed")) {
		t.Fatal("permission failure classified as not found")
	}
}
