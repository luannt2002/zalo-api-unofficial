package api

import (
	"testing"

	"github.com/luannt2002/zalo-api-unofficial/internal/testenv"
	"github.com/luannt2002/zalo-api-unofficial/session"
)

func TestSend2meHelpers(t *testing.T) {
	env := testenv.Load(t)
	sc := session.NewContext(session.WithLogging(false))
	sc.SealLogin(session.Seal{
		UID: env.UIDA,
		LoginInfo: &session.LoginInfo{
			Send2meID: env.Send2meA,
		},
	})

	a := &api{sc: sc}

	if got := a.GetSend2meID(); got != env.Send2meA {
		t.Fatalf("expected send2me_id %q, got %q", env.Send2meA, got)
	}

	if !a.IsSend2me(env.Send2meA) {
		t.Fatal("expected IsSend2me to return true for matching id")
	}

	if a.IsSend2me("other_id") {
		t.Fatal("expected IsSend2me to return false for different id")
	}

	if a.IsSend2me("") {
		t.Fatal("expected IsSend2me to return false for empty id")
	}
}
