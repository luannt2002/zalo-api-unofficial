package api

import (
	"context"
	"testing"

	"github.com/luannt2002/zalo-api-unofficial/session"
)

func TestRenewLinkValidation(t *testing.T) {
	sc := session.NewContext(session.WithLogging(false))
	// No login info sealed -> send2me_id is empty
	a := &api{sc: sc}

	fn, err := renewLinkFactory(sc, a)
	if err != nil {
		t.Fatalf("failed to create factory: %v", err)
	}

	ctx := context.Background()

	// Empty toId and no send2me_id
	if _, err := fn(ctx, RenewLinkRequest{}); err == nil {
		t.Fatal("expected error on empty toId with no send2me context")
	}
}

