package gid

import (
	"strconv"
	"testing"

	"github.com/luannt2002/zalo-api-unofficial/internal/testenv"
)

func TestMaskFromSend2me(t *testing.T) {
	// Nạp và xác thực các ID kiểm thử từ environment (.env.test)
	env := testenv.Load(t)
	send2meA := env.Send2meA
	send2meB := env.Send2meB
	expectedMask := env.ExpectedMask

	mask, err := MaskFromSend2me(send2meA, send2meB)
	if err != nil {
		t.Fatalf("MaskFromSend2me returned error: %v", err)
	}
	if mask != expectedMask {
		t.Fatalf("Expected mask 0x%x, got 0x%x", expectedMask, mask)
	}

	// Đổi vị trí A và B kết quả mask đối xứng phải bằng nhau (A ^ B == B ^ A)
	reverseMask, err := MaskFromSend2me(send2meB, send2meA)
	if err != nil {
		t.Fatalf("Reverse MaskFromSend2me error: %v", err)
	}
	if reverseMask != mask {
		t.Fatalf("Mask not symmetric: %x vs %x", reverseMask, mask)
	}
}

func TestTranslateViewerUID(t *testing.T) {
	env := testenv.Load(t)
	send2meA := env.Send2meA
	send2meC := env.Send2meC
	uidEntity := env.UIDEntity

	// Entity test:
	uA, _ := strconv.ParseUint(uidEntity, 10, 64)
	mask, _ := MaskFromSend2me(send2meA, send2meC)
	expectedUB := uA ^ mask

	translated, err := TranslateViewerUID(uidEntity, send2meA, send2meC)
	if err != nil {
		t.Fatalf("TranslateViewerUID error: %v", err)
	}
	if translated != strconv.FormatUint(expectedUB, 10) {
		t.Fatalf("Expected %s, got %s", strconv.FormatUint(expectedUB, 10), translated)
	}

	// Round-trip C -> A phải ra lại đúng uidEntity
	roundTrip, err := TranslateViewerUID(translated, send2meC, send2meA)
	if err != nil {
		t.Fatalf("Roundtrip error: %v", err)
	}
	if roundTrip != uidEntity {
		t.Fatalf("Roundtrip mismatch: expected %s, got %s", uidEntity, roundTrip)
	}
}

func TestStore(t *testing.T) {
	env := testenv.Load(t)
	s := NewStore()
	s.Set("u123", "g456")
	s.Set("u789", "g999")

	if s.Len() != 2 {
		t.Fatalf("Expected len 2, got %d", s.Len())
	}

	g, ok := s.GetGID("u123")
	if !ok || g != "g456" {
		t.Fatalf("Expected g456, got %s", g)
	}

	u, ok := s.GetUID("g456")
	if !ok || u != "u123" {
		t.Fatalf("Expected u123, got %s", u)
	}

	_, ok = s.GetGID("nonexistent")
	if ok {
		t.Fatal("Expected false for nonexistent key")
	}

	// Test GetOrComputeMask caching with env-loaded identities
	mask1, err := s.GetOrComputeMask(env.Send2meA, env.Send2meB)
	if err != nil || mask1 != env.ExpectedMask {
		t.Fatalf("expected mask 0x%x, got 0x%x, err: %v", env.ExpectedMask, mask1, err)
	}

	// Call again reversed: should hit cache
	mask2, err := s.GetOrComputeMask(env.Send2meB, env.Send2meA)
	if err != nil || mask2 != env.ExpectedMask {
		t.Fatalf("expected cached mask 0x%x, got 0x%x, err: %v", env.ExpectedMask, mask2, err)
	}
}

func TestCalibrateMaskAndTranslateGroupID(t *testing.T) {
	env := testenv.Load(t)
	send2meA := env.Send2meA
	send2meB := env.Send2meB
	expectedMask := env.ExpectedMask

	// 1. CalibrateMask between send2meA and send2meB
	calibrated, err := CalibrateMask(send2meA, send2meB)
	if err != nil {
		t.Fatalf("CalibrateMask error: %v", err)
	}
	if calibrated != expectedMask {
		t.Fatalf("Calibrated mask 0x%x != expected 0x%x", calibrated, expectedMask)
	}

	// 2. TranslateGroupID using mask
	groupID := "8910587406831154166"
	translatedGroupID, err := TranslateGroupID(groupID, calibrated)
	if err != nil {
		t.Fatalf("TranslateGroupID error: %v", err)
	}

	// Round-trip
	restored, err := TranslateGroupID(translatedGroupID, calibrated)
	if err != nil {
		t.Fatalf("Roundtrip TranslateGroupID error: %v", err)
	}
	if restored != groupID {
		t.Fatalf("Restored groupID %q != %q", restored, groupID)
	}
}

