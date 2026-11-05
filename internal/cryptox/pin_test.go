package cryptox

import "testing"

func TestPIN(t *testing.T) {
	pin := "1234"
	enc := EncryptPIN(pin)
	if len(enc) != 32 {
		t.Fatalf("expected 32 hex chars, got %d", len(enc))
	}

	if !ValidatePIN(enc, "1234") {
		t.Fatal("expected pin 1234 to be valid")
	}

	if ValidatePIN(enc, "5678") {
		t.Fatal("expected pin 5678 to be invalid")
	}
}
