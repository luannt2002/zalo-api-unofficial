package listener

import (
	"context"
	"testing"
)

// Bốn kênh mới (UnreadCleared, PinConversations, UnreadMark, Voip) là kênh tuỳ chọn như Unhandled:
// ứng dụng cũ không đọc chúng thì không được đẩy DroppedEvents (báo động giả "mất tin"), nhưng
// tin rơi vẫn phải đếm được (DroppedOptionalEvents) cho ứng dụng có đọc mà đọc chậm.
func TestKenhTuyChonKhongDayDroppedEvents(t *testing.T) {
	frames := []struct {
		name string
		cmd  uint
		body BaseWSMessage
	}{
		{"UnreadCleared", 504, plainFrame(`{"clearUnreads":[{"idTo":"300","lastMsgId":"1","type":0}]}`)},
		{"PinConversations", 601, controlFrame(`{"act_type":"pinned_convers","act":"update","data":{"tab":0,"action":1,"pinIds":["u1"],"ts":1}}`)},
		{"UnreadMark", 601, controlFrame(`{"act_type":"mark_unread","act":"add","content":{"convsUser":[{"id":"200"}]}}`)},
		{"Voip", 601, controlFrame(`{"act_type":"voip","act":"request","data":{"callId":1}}`)},
	}
	for _, f := range frames {
		t.Run(f.name, func(t *testing.T) {
			ln := newRouterTestListener(t)
			dropped, optional := DroppedEvents(), DroppedOptionalEvents()
			for range 200 {
				ln.router(context.Background(), 1, f.cmd, 0, f.body)
			}
			if got := DroppedEvents() - dropped; got != 0 {
				t.Fatalf("DroppedEvents tăng %d khi không ai đọc kênh %s, mong 0", got, f.name)
			}
			if got, want := DroppedOptionalEvents()-optional, int64(200-64); got != want {
				t.Fatalf("DroppedOptionalEvents tăng %d, mong %d (200 - buffer 64)", got, want)
			}
		})
	}
}

// Unhandled theo cùng chính sách: tràn thì đếm vào DroppedOptionalEvents.
func TestUnhandledTranDemVaoKenhTuyChon(t *testing.T) {
	ln := newRouterTestListener(t)
	optional := DroppedOptionalEvents()
	for range 100 {
		ln.router(context.Background(), 1, 999, 0, plainFrame(`{}`))
	}
	if got, want := DroppedOptionalEvents()-optional, int64(100-64); got != want {
		t.Fatalf("DroppedOptionalEvents tăng %d, mong %d", got, want)
	}
}
