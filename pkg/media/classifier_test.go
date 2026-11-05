package media

import (
	"testing"
	"time"
)

func TestClassifyLegacyAvatar(t *testing.T) {
	rawURL := "https://s160-ava-talk.zadn.vn/2/f/3/7/15/160/4baa84b4b71ee28b0666e177908f3d4d.jpg"
	res := Classify(rawURL)

	if !res.OK {
		t.Fatalf("expected OK true, got false")
	}
	if res.HostClass != HostClassZaloCDN {
		t.Fatalf("expected HostClassZaloCDN, got: %s", res.HostClass)
	}
	if res.PathSegments != 7 {
		t.Fatalf("expected 7 segments, got: %d", res.PathSegments)
	}
	if res.Class != ClassUnverifiedURL {
		t.Fatalf("expected ClassUnverifiedURL for unsigned static avatar, got: %s", res.Class)
	}

	shape := Shape(rawURL)
	expectedShape := "https://s160-ava-talk.zadn.vn/<7-segments>"
	if shape != expectedShape {
		t.Fatalf("expected shape %s, got %s", expectedShape, shape)
	}
}

func TestClassifySignedAvatar(t *testing.T) {
	rawURL := "https://s240-26-ava-talk.zadn.vn/avatar.jpg?key=abcdefghijklmnopqrstuv&time=1795107600"
	res := Classify(rawURL)

	if !res.OK {
		t.Fatalf("expected OK true, got false")
	}
	if res.HostClass != HostClassZaloCDN {
		t.Fatalf("expected HostClassZaloCDN, got: %s", res.HostClass)
	}
	if !res.SignatureHint {
		t.Fatalf("expected SignatureHint true")
	}
	if res.Class != ClassCapabilityCandidate {
		t.Fatalf("expected ClassCapabilityCandidate, got: %s", res.Class)
	}
	if res.ExpiresAt == nil {
		t.Fatalf("expected ExpiresAt timestamp, got nil")
	}

	expectedEpoch := time.Unix(1795107600, 0).UTC()
	if !res.ExpiresAt.Equal(expectedEpoch) {
		t.Fatalf("ExpiresAt mismatch: got %v, want %v", res.ExpiresAt, expectedEpoch)
	}
}

func TestClassifyInvalidURL(t *testing.T) {
	res := Classify("not-a-valid-url")
	if res.OK {
		t.Fatalf("expected invalid URL to fail")
	}
	if Shape("not-a-valid-url") != "INVALID_URL" {
		t.Fatalf("expected INVALID_URL shape")
	}
}

