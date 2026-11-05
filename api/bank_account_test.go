package api

import (
	"context"
	"testing"

	"github.com/luannt2002/zalo-api-unofficial/model"
	"github.com/luannt2002/zalo-api-unofficial/session"
)

func TestBankAccountValidation(t *testing.T) {
	sc := session.NewContext()
	sc.SealLogin(session.Seal{
		UID:       "12345",
		UserAgent: "test",
		SecretKey: session.SecretKey("dGVzdC1zZWNyZXQta2V5LTE2Yg=="),
		Settings:  &session.Settings{},
	})

	createFn, err := createBankAccountFactory(sc, &api{sc: sc})
	if err != nil {
		t.Fatalf("createBankAccountFactory failed: %v", err)
	}

	// Name too short (< 5 chars)
	_, err = createFn(context.Background(), CreateBankAccountRequest{
		BinBank:     model.BinBankTechcombank,
		NumAccBank:  "19031234567890",
		NameAccBank: "AB",
	})
	if err == nil {
		t.Fatal("expected error on short account name")
	}

	updateFn, err := updateBankAccountFactory(sc, &api{sc: sc})
	if err != nil {
		t.Fatalf("updateBankAccountFactory failed: %v", err)
	}

	_, err = updateFn(context.Background(), UpdateBankAccountRequest{
		AccountID:   123,
		BinBank:     model.BinBankTechcombank,
		NumAccBank:  "19031234567890",
		NameAccBank: "",
	})
	if err == nil {
		t.Fatal("expected error on empty account name")
	}
}
