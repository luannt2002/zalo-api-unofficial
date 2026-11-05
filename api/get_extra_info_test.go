package api

import (
	"context"
	"testing"

	"github.com/luannt2002/zalo-api-unofficial/session"
)

func TestGetExtraFriendInfoValidation(t *testing.T) {
	sc := session.NewContext()
	sc.SealLogin(session.Seal{
		UID:       "12345",
		UserAgent: "test",
		SecretKey: session.SecretKey("dGVzdC1zZWNyZXQta2V5LTE2Yg=="),
		Settings:  &session.Settings{},
	})

	fn, err := getExtraFriendInfoFactory(sc, &api{sc: sc})
	if err != nil {
		t.Fatalf("factory failed: %v", err)
	}

	if _, err := fn(context.Background()); err == nil {
		t.Fatal("expected error on empty friendIDs")
	}
}

func TestGetExtraGroupInfoValidation(t *testing.T) {
	sc := session.NewContext()
	sc.SealLogin(session.Seal{
		UID:       "12345",
		UserAgent: "test",
		SecretKey: session.SecretKey("dGVzdC1zZWNyZXQta2V5LTE2Yg=="),
		Settings:  &session.Settings{},
	})

	fn, err := getExtraGroupInfoFactory(sc, &api{sc: sc})
	if err != nil {
		t.Fatalf("factory failed: %v", err)
	}

	if _, err := fn(context.Background()); err == nil {
		t.Fatal("expected error on empty groupIDs")
	}
}
