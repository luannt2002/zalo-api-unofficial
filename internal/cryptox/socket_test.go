package cryptox

import (
	"encoding/hex"
	"testing"
)

func TestSocketKeyDerivation(t *testing.T) {
	sequence := int32(1)
	privateKey := int64(123456789012345678)

	key := DeriveSocketKey(sequence, privateKey)
	keyHex := hex.EncodeToString(key)

	expectedHex := "0000000101b69b4ba630f34e48ed3a1f52f93941e3210d3f2e5de4101d1f5b30"
	if keyHex != expectedHex {
		t.Fatalf("DeriveSocketKey mismatch:\n  got:  %s\n  want: %s", keyHex, expectedHex)
	}

	if len(key) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(key))
	}
}

func TestEncodeDecodeSocketParams(t *testing.T) {
	sequence := int32(1)
	privateKey := int64(123456789012345678)
	plaintext := "test=123"

	encoded, err := EncodeSocketParams(sequence, privateKey, plaintext)
	if err != nil {
		t.Fatalf("EncodeSocketParams failed: %v", err)
	}

	expectedB64 := "Myjho46EUl5y7xAByYzp0Q=="
	if encoded != expectedB64 {
		t.Fatalf("EncodeSocketParams result mismatch:\n  got:  %s\n  want: %s", encoded, expectedB64)
	}

	decoded, err := DecodeSocketParams(sequence, privateKey, encoded)
	if err != nil {
		t.Fatalf("DecodeSocketParams failed: %v", err)
	}

	if decoded != plaintext {
		t.Fatalf("DecodeSocketParams roundtrip mismatch:\n  got:  %s\n  want: %s", decoded, plaintext)
	}
}

