package cryptox

import (
	"strings"
	"testing"
)

func TestDeriveHttpKeyFromZCID(t *testing.T) {
	// zcid 32 hex chars
	zcid := "0123456789ABCDEF0123456789ABCDEF"
	key, err := DeriveHttpKeyFromZCID(zcid)
	if err != nil {
		t.Fatalf("DeriveHttpKeyFromZCID failed: %v", err)
	}

	if len(key) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(key))
	}

	// Chẵn: 0, 2, 4, 6, 8, A, C, E, 0, 2, 4, 6, 8, A, C, E
	// Lẻ đảo chiều: F, D, B, 9, 7, 5, 3, 1, F, D, B, 9, 7, 5, 3, 1
	expectedKey := "02468ACE02468ACEFDB97531FDB97531"
	if string(key) != expectedKey {
		t.Fatalf("key mismatch:\n got:  %s\n want: %s", string(key), expectedKey)
	}
}

func TestMobileHttpParamsRoundTrip(t *testing.T) {
	zcid := "0123456789ABCDEF0123456789ABCDEF"
	plain := "action=sync&seq=100&user=test"

	encrypted, err := EncryptMobileHttpParams(plain, zcid)
	if err != nil {
		t.Fatalf("EncryptMobileHttpParams failed: %v", err)
	}

	if encrypted != strings.ToUpper(encrypted) {
		t.Fatalf("expected uppercase hex ciphertext, got: %s", encrypted)
	}

	decrypted, err := DecryptMobileHttpParams(encrypted, zcid)
	if err != nil {
		t.Fatalf("DecryptMobileHttpParams failed: %v", err)
	}

	if decrypted != plain {
		t.Fatalf("roundtrip mismatch: got %s, want %s", decrypted, plain)
	}
}

