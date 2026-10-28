package listener

import (
	"context"
	"testing"
	"time"

	"github.com/luannt2002/zalo-api-unofficial/model"
	"github.com/luannt2002/zalo-api-unofficial/session"
)

const testSelfUID = "100"

// newRouterTestListener dựng listener không mạng để bơm khung thẳng vào router.
func newRouterTestListener(t *testing.T) *listener {
	t.Helper()
	sc := session.NewContext(session.WithLogging(false))
	sc.SealLogin(session.Seal{UID: testSelfUID})
	return &listener{
		sc:              sc,
		ch:              initializeChannels(),
		retryStates:     map[string]*retryState{},
		fallbackRetries: make(map[int]int),
	}
}

// plainFrame bọc payload thành khung không mã hoá (encrypt=0) như máy chủ gửi: {"data": payload}.
func plainFrame(payload string) BaseWSMessage {
	return BaseWSMessage{Encrypt: EncryptionTypeNone, Data: `{"data":` + payload + `}`}
}

func recvOne[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(time.Second):
		var zero T
		t.Fatalf("không nhận được sự kiện %T trong 1s", zero)
		return zero
	}
}

func assertEmpty[T any](t *testing.T, ch <-chan T) {
	t.Helper()
	select {
	case v := <-ch:
		t.Fatalf("không mong sự kiện, nhận %#v", v)
	default:
	}
}

// cmd 551 mang tin 1-1 giống 501 (upstream PR #378); trước đây router bỏ im lặng.
func TestRouterCmd551LaTinMotMot(t *testing.T) {
	ln := newRouterTestListener(t)

	ln.router(context.Background(), 1, 551, 0, plainFrame(
		`{"msgs":[{"msgId":"m551","uidFrom":"200","idTo":"0","content":"xin chào"}]}`))

	msg := recvOne(t, ln.Message())
	um, ok := msg.(model.UserMessage)
	if !ok {
		t.Fatalf("Message() = %T, mong model.UserMessage", msg)
	}
	if um.Data.MsgID != "m551" || um.ThreadID() != "200" {
		t.Fatalf("msgId=%q threadId=%q, mong m551/200", um.Data.MsgID, um.ThreadID())
	}
}
