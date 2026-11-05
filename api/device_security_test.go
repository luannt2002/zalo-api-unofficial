package api

import (
	"context"
	"testing"

	"github.com/luannt2002/zalo-api-unofficial/session"
)

func TestScanURLValidation(t *testing.T) {
	sc := session.NewContext()
	sc.SealLogin(session.Seal{
		UID:       "12345",
		UserAgent: "test",
		SecretKey: session.SecretKey("dGVzdC1zZWNyZXQta2V5LTE2Yg=="),
		Settings:  &session.Settings{},
	})

	scanFn, err := scanURLFactory(sc, &api{sc: sc})
	if err != nil {
		t.Fatalf("scanURLFactory failed: %v", err)
	}

	_, err = scanFn(context.Background(), "")
	if err == nil {
		t.Fatal("expected error on empty url")
	}
}
