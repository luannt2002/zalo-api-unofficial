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

func TestRenewLinkWaitSyncReturn(t *testing.T) {
	a := &api{
		e: endpoints{
			RenewLink: func(ctx context.Context, req RenewLinkRequest) (*RenewLinkResponse, error) {
				return &RenewLinkResponse{
					Async:  false,
					Normal: "http://renewed/normal.jpg",
				}, nil
			},
		},
	}

	res, err := a.RenewLinkWait(context.Background(), RenewLinkRequest{ToID: "123"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Normal != "http://renewed/normal.jpg" {
		t.Fatalf("expected renewed normal url, got: %s", res.Normal)
	}
}

