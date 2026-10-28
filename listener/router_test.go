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

// cmd 504 (1-1) / 524 (nhóm) subCmd 0: hội thoại đã đọc ở thiết bị khác (upstream PR #380).
// Chỉ type 0 là "đọc hội thoại"; type 2 đi kèm idTo "-1" nên bị lọc.
func TestRouterUnreadCleared(t *testing.T) {
	tests := []struct {
		name     string
		cmd      uint
		wantType model.ThreadType
	}{
		{"504 user", 504, model.ThreadTypeUser},
		{"524 group", 524, model.ThreadTypeGroup},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ln := newRouterTestListener(t)
			ln.router(context.Background(), 1, tt.cmd, 0, plainFrame(`{"clearUnreads":[
				{"idTo":"300","isGroup":0,"lastMsgId":"7001","type":0,"ts":1},
				{"idTo":400,"lastMsgId":7002,"type":0},
				{"idTo":"-1","type":2},
				{"idTo":"","type":0}
			]}`))

			items := recvOne(t, ln.UnreadCleared())
			if len(items) != 2 {
				t.Fatalf("len(items) = %d, mong 2 (chỉ type 0 có idTo): %#v", len(items), items)
			}
			want := []struct{ thread, last string }{{"300", "7001"}, {"400", "7002"}}
			for i, w := range want {
				it := items[i]
				if it.Type != tt.wantType || it.ThreadID != w.thread || it.LastMsgID != w.last {
					t.Errorf("items[%d] = {%v %q %q}, mong {%v %q %q}", i, it.Type, it.ThreadID, it.LastMsgID, tt.wantType, w.thread, w.last)
				}
				if len(it.Data) == 0 {
					t.Errorf("items[%d].Data rỗng, mong giữ bản thô", i)
				}
			}
		})
	}
}

func TestRouterUnreadClearedKhongPhatKhiRong(t *testing.T) {
	ln := newRouterTestListener(t)
	ln.router(context.Background(), 1, 504, 0, plainFrame(`{"clearUnreads":[{"idTo":"-1","type":2}]}`))
	ln.router(context.Background(), 1, 504, 0, plainFrame(`{"clearUnreads":[]}`))
	assertEmpty(t, ln.UnreadCleared())
}
