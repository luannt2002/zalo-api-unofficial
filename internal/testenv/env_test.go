package testenv

import "testing"

func TestLoadAndVerify(t *testing.T) {
	id := Load(t)

	if id.Send2meA == "" || id.Send2meB == "" || id.UIDA == "" {
		t.Fatalf("expected non-empty test identities: %+v", id)
	}

	if id.ExpectedMask == 0 {
		t.Fatal("expected non-zero ExpectedMask")
	}

	// Verify idempotency
	id2 := Load(t)
	if id.Send2meA != id2.Send2meA || id.ExpectedMask != id2.ExpectedMask {
		t.Fatal("expected cached identities on subsequent calls")
	}
}

