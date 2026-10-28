package listener

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/luannt2002/zalo-api-unofficial/model"
)

// Một tin hỏng trong lô KHÔNG được làm mất cả lô, và KHÔNG được biến mất im lặng:
// tin hỏng bị bỏ + đếm (DecodeFailures) + phát *DecodeError kèm msgId và khung thô cắt ngắn.
// content là số ⇒ không khớp kiểu content nào ⇒ tin "bad" không giải được.

func expectDecodeError(t *testing.T, ln *listener, wantMsgID string) {
	t.Helper()
	err := recvOne(t, ln.Error())
	var de *DecodeError
	if !errors.As(err, &de) {
		t.Fatalf("Error() = %T %v, mong *DecodeError", err, err)
	}
	if de.MsgID != wantMsgID || !strings.Contains(de.Raw, wantMsgID) || de.Err == nil {
		t.Fatalf("DecodeError = %+v, mong msgId %q và raw chứa nó", de, wantMsgID)
	}
	if !strings.Contains(err.Error(), wantMsgID) {
		t.Fatalf("Error() = %q, phải in msgId", err.Error())
	}
}

func TestDecodeItemMessagesLoHong(t *testing.T) {
	tests := []struct {
		name  string
		cmd   uint
		frame string
	}{
		{"501", 501, `{"msgs":[{"msgId":"ok1","uidFrom":"200","idTo":"0","content":"a"},{"msgId":"bad","uidFrom":"200","idTo":"0","content":12},{"msgId":"ok2","uidFrom":"200","idTo":"0","content":"b"}]}`},
		{"521", 521, `{"groupMsgs":[{"msgId":"ok1","uidFrom":"200","idTo":"900","content":"a"},{"msgId":"bad","uidFrom":"200","idTo":"900","content":12},{"msgId":"ok2","uidFrom":"200","idTo":"900","content":"b"}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ln := newRouterTestListener(t)
			before := DecodeFailures()
			ln.router(context.Background(), 1, tt.cmd, 0, plainFrame(tt.frame))

			for _, want := range []string{"ok1", "ok2"} {
				var id string
				switch m := recvOne(t, ln.Message()).(type) {
				case model.UserMessage:
					id = m.Data.MsgID
				case model.GroupMessage:
					id = m.Data.MsgID
				}
				if id != want {
					t.Fatalf("msgId = %q, mong %q", id, want)
				}
			}
			expectDecodeError(t, ln, "bad")
			if got := DecodeFailures() - before; got != 1 {
				t.Fatalf("DecodeFailures tăng %d, mong 1", got)
			}
		})
	}
}

func TestDecodeItemOldMessagesLoHong(t *testing.T) {
	ln := newRouterTestListener(t)
	ln.router(context.Background(), 1, 510, 1, plainFrame(
		`{"msgs":[{"msgId":"ok1","uidFrom":"200","idTo":"0","content":"a"},{"msgId":"bad","content":12},{"msgId":"ok2","uidFrom":"200","idTo":"0","content":"b"}]}`))

	old := recvOne(t, ln.OldMessages())
	if len(old.Messages) != 2 || old.ThreadType != model.ThreadTypeUser {
		t.Fatalf("OldMessages = %d tin type %v, mong 2 tin user", len(old.Messages), old.ThreadType)
	}
	expectDecodeError(t, ln, "bad")
}

func TestDecodeItemReactionsLoHong(t *testing.T) {
	frame := `{"reacts":[
		{"msgId":"ok1","uidFrom":"200","idTo":"0","content":"{\"rType\":5,\"rIcon\":\"/-heart\",\"rMsg\":[]}"},
		{"msgId":"bad","uidFrom":"200","idTo":"0","content":"{khong phai json"},
		{"msgId":"ok2","uidFrom":"200","idTo":"0","content":"{\"rType\":5,\"rIcon\":\"/-heart\",\"rMsg\":[]}"}
	]}`

	t.Run("612", func(t *testing.T) {
		ln := newRouterTestListener(t)
		ln.router(context.Background(), 1, 612, 0, plainFrame(frame))
		for _, want := range []string{"ok1", "ok2"} {
			if r := recvOne(t, ln.Reaction()); r.Data.MsgID != want {
				t.Fatalf("reaction msgId = %q, mong %q", r.Data.MsgID, want)
			}
		}
		expectDecodeError(t, ln, "bad")
	})
	t.Run("610", func(t *testing.T) {
		ln := newRouterTestListener(t)
		ln.router(context.Background(), 1, 610, 1, plainFrame(frame))
		if old := recvOne(t, ln.OldReactions()); len(old.Reactions) != 2 {
			t.Fatalf("OldReactions = %d, mong 2", len(old.Reactions))
		}
		expectDecodeError(t, ln, "bad")
	})
}

// Raw bị cắt để một tin khổng lồ không thổi phồng lỗi.
func TestDecodeItemRawCatNgan(t *testing.T) {
	ln := newRouterTestListener(t)
	big := strings.Repeat("x", 5000)
	ln.router(context.Background(), 1, 501, 0, plainFrame(`{"msgs":[{"msgId":"bad","content":12,"pad":"`+big+`"}]}`))
	var de *DecodeError
	if err := recvOne(t, ln.Error()); !errors.As(err, &de) || len(de.Raw) > 600 {
		t.Fatalf("err = %v, len(Raw) = %d, mong *DecodeError và Raw <= 600", err, len(de.Raw))
	}
}
