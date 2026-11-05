package api

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/luannt2002/zalo-api-unofficial/internal/testenv"
	"github.com/luannt2002/zalo-api-unofficial/session"
)

func TestTranslateViewerUIDOffline(t *testing.T) {
	env := testenv.Load(t)
	send2meA := env.Send2meA
	send2meB := env.Send2meB
	uidA := env.UIDA

	sc := session.NewContext(session.WithLogging(false))
	sc.SealLogin(session.Seal{
		UID: uidA,
		LoginInfo: &session.LoginInfo{
			Send2meID: send2meA,
		},
	})

	a := &api{sc: sc}
	fn, err := translateViewerUIDFactory(sc, a)
	if err != nil {
		t.Fatalf("failed to create factory: %v", err)
	}

	uidAUint, _ := strconv.ParseUint(uidA, 10, 64)
	expectedMask := env.ExpectedMask
	expectedUIDB := fmt.Sprintf("%d", uidAUint^expectedMask)

	resp, err := fn(context.Background(), TranslateViewerUIDRequest{
		UIDs:          []string{uidA},
		TargetSend2me: send2meB,
	})
	if err != nil {
		t.Fatalf("TranslateViewerUID failed: %v", err)
	}

	if resp.Mask != expectedMask {
		t.Fatalf("expected mask 0x%x, got 0x%x", expectedMask, resp.Mask)
	}

	if got := resp.Mapping[uidA]; got != expectedUIDB {
		t.Fatalf("expected translated UID %s, got %s", expectedUIDB, got)
	}
}

func TestTranslateViewerUIDValidation(t *testing.T) {
	sc := session.NewContext(session.WithLogging(false))
	a := &api{sc: sc}
	fn, err := translateViewerUIDFactory(sc, a)
	if err != nil {
		t.Fatalf("failed to create factory: %v", err)
	}

	ctx := context.Background()

	// Empty UIDs
	if _, err := fn(ctx, TranslateViewerUIDRequest{}); err == nil {
		t.Fatal("expected error on empty UIDs")
	}

	// Missing send2me
	if _, err := fn(ctx, TranslateViewerUIDRequest{UIDs: []string{"123"}}); err == nil {
		t.Fatal("expected error on missing send2me")
	}
}

func TestConvertGlobalIDValidation(t *testing.T) {
	sc := session.NewContext(session.WithLogging(false))
	a := &api{sc: sc}
	fn, err := convertGlobalIDFactory(sc, a)
	if err != nil {
		t.Fatalf("failed to create factory: %v", err)
	}

	ctx := context.Background()

	// Empty IDs
	if _, err := fn(ctx, ConvertGlobalIDRequest{}); err == nil {
		t.Fatal("expected error on empty IDs")
	}

	// Invalid direction
	if _, err := fn(ctx, ConvertGlobalIDRequest{IDs: []string{"123"}, Direction: "invalid"}); err == nil {
		t.Fatal("expected error on invalid direction")
	}
}

func TestGIDEnvelopeUnwrap(t *testing.T) {
	env := testenv.Load(t)
	uid := env.UIDA
	gid := "0123456789ABCDEFGHIJKLMNOPQRSTUV"

	// 1. Encrypt flat vs nested
	flatJSON := fmt.Sprintf(`{"%s": "%s"}`, uid, gid)
	var encFlat EncryptGlobalIDResponse
	if err := encFlat.UnmarshalJSON([]byte(flatJSON)); err != nil {
		t.Fatalf("failed to unmarshal flat encrypt response: %v", err)
	}
	if encFlat[uid] != gid {
		t.Fatalf("unexpected flat encrypt mapping: %v", encFlat)
	}

	nestedJSON := fmt.Sprintf(`{"data": {"%s": "%s"}}`, uid, gid)
	var encNested EncryptGlobalIDResponse
	if err := encNested.UnmarshalJSON([]byte(nestedJSON)); err != nil {
		t.Fatalf("failed to unmarshal nested encrypt response: %v", err)
	}
	if encNested[uid] != gid {
		t.Fatalf("unexpected nested encrypt mapping: %v", encNested)
	}

	// 2. Decrypt flat vs nested
	flatDecJSON := fmt.Sprintf(`{"%s": "%s"}`, gid, uid)
	var decFlat DecryptGlobalIDResponse
	if err := decFlat.UnmarshalJSON([]byte(flatDecJSON)); err != nil {
		t.Fatalf("failed to unmarshal flat decrypt response: %v", err)
	}
	if decFlat[gid] != uid {
		t.Fatalf("unexpected flat decrypt mapping: %v", decFlat)
	}

	nestedDecJSON := fmt.Sprintf(`{"data": {"%s": "%s"}}`, gid, uid)
	var decNested DecryptGlobalIDResponse
	if err := decNested.UnmarshalJSON([]byte(nestedDecJSON)); err != nil {
		t.Fatalf("failed to unmarshal nested decrypt response: %v", err)
	}
	if decNested[gid] != uid {
		t.Fatalf("unexpected nested decrypt mapping: %v", decNested)
	}
}
