package gid

import (
	"testing"
	"time"

	"github.com/luannt2002/zalo-api-unofficial/internal/testenv"
)

func TestSnowflakeUIDToTime(t *testing.T) {
	env := testenv.Load(t)
	uid := env.UIDA
	ts, err := UIDToTime(uid)
	if err != nil {
		t.Fatalf("UIDToTime error: %v", err)
	}

	// Thời điểm tạo phải sau Epoch (2011) và trước tương lai xa
	if ts.Before(SnowflakeEpoch) || ts.After(time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected timestamp: %v", ts)
	}

	bit63, randBit15, err := UIDBits(uid)
	if err != nil {
		t.Fatalf("UIDBits error: %v", err)
	}
	if !bit63 {
		t.Fatal("expected bit63 == true (bit63 is 0)")
	}
	_ = randBit15
}

func TestIsWellFormedGID(t *testing.T) {
	// GID hợp lệ 32 ký tự, pad byte = 0
	// 32 ký tự '0' chắc chắn có pad byte = 0
	validAllZeros := "00000000000000000000000000000000"
	if !IsWellFormedGID(validAllZeros) {
		t.Fatal("expected all zeros 32 chars to be well-formed")
	}

	// Quá ngắn
	if IsWellFormedGID("00000") {
		t.Fatal("expected short GID to be invalid")
	}

	// Ký tự không thuộc bảng GIDAlphabet (ví dụ 'Z')
	invalidChar := "0000000000000000000000000000000Z"
	if IsWellFormedGID(invalidChar) {
		t.Fatal("expected invalid char GID to be invalid")
	}
}

func TestToolCol3Roundtrip(t *testing.T) {
	env := testenv.Load(t)
	uid := env.UIDA
	col3, err := UIDToToolCol3(uid)
	if err != nil {
		t.Fatalf("UIDToToolCol3 error: %v", err)
	}

	decoded, err := ToolCol3ToUID(col3)
	if err != nil {
		t.Fatalf("ToolCol3ToUID error: %v", err)
	}

	if decoded != uid {
		t.Fatalf("expected roundtrip %q, got %q", uid, decoded)
	}
}

func TestGIDBlobAndCohortWindow(t *testing.T) {
	env := testenv.Load(t)
	gidStr := env.GIDA
	if gidStr == "" {
		t.Skip("TEST_ZALO_GIDA not set in environment")
	}

	// 1. GID to blob and back round-trip
	blob, err := GIDToBlob(gidStr)
	if err != nil {
		t.Fatalf("GIDToBlob error: %v", err)
	}

	reconstructed := BlobToGID(blob)
	if reconstructed != gidStr {
		t.Fatalf("BlobToGID = %q, expected %q", reconstructed, gidStr)
	}

	// 2. Pad byte check
	if !IsWellFormedGID(gidStr) {
		t.Fatalf("expected GID %q to be well-formed", gidStr)
	}

	// 3. Payload and cohort window
	payload := GIDPayloadBytes(blob)
	if len(payload) != 19 {
		t.Fatalf("payload length = %d, expected 19", len(payload))
	}

	w := GIDCohortWindow(blob)
	_ = w
}
