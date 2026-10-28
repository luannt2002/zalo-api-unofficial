package listener

import (
	"context"
	"slices"
	"testing"

	"github.com/luannt2002/zalo-api-unofficial/model"
)

func oldMsgIDs(t *testing.T, old model.OldMessages) []string {
	t.Helper()
	ids := []string{}
	for _, m := range old.Messages {
		switch v := m.(type) {
		case model.UserMessage:
			ids = append(ids, v.Data.MsgID)
		case model.GroupMessage:
			ids = append(ids, v.Data.MsgID)
		default:
			t.Fatalf("tin cũ kiểu %T", m)
		}
	}
	return ids
}

const (
	oldText1   = `{"msgId":"1","cliMsgId":"c1","msgType":"webchat","uidFrom":"200","idTo":"0","ts":"1761089990000","content":"hello"}`
	oldText2   = `{"msgId":"2","cliMsgId":"c2","msgType":"webchat","uidFrom":"200","idTo":"0","ts":"1761089990000","content":"hello"}`
	oldRecall1 = `{"msgId":"r1","cliMsgId":"rc1","msgType":"chat.undo","uidFrom":"200","idTo":"0","ts":"1761090000000","content":{"globalMsgId":"1","cliMsgId":"1761089990001","deleteMsg":0,"srcId":"0","destId":"0"}}`
)

// 5(a) — tin cũ của nhóm (cmd 511) phải là GroupMessage trên thread nhóm (upstream: new GroupMessage).
// Bản cũ dựng UserMessage ⇒ ThreadID = người gửi, ứng dụng phải tự sửa thành idTo.
func TestOldMessagesNhomLaGroupMessage(t *testing.T) {
	ln := newRouterTestListener(t)
	ln.router(context.Background(), 1, 511, 1, plainFrame(`{"groupMsgs":[
		{"msgId":"g1","uidFrom":"200","idTo":"900","content":"a","mentions":[{"uid":"201","pos":0,"len":2,"type":0}]}]}`))

	old := recvOne(t, ln.OldMessages())
	if old.ThreadType != model.ThreadTypeGroup || len(old.Messages) != 1 {
		t.Fatalf("OldMessages = %d tin type %v, mong 1 tin nhóm", len(old.Messages), old.ThreadType)
	}
	gm, ok := old.Messages[0].(model.GroupMessage)
	if !ok {
		t.Fatalf("tin cũ nhóm = %T, mong model.GroupMessage", old.Messages[0])
	}
	if gm.ThreadID() != "900" || len(gm.Data.Mentions) != 1 {
		t.Fatalf("ThreadID = %q mentions = %d, mong 900 / 1", gm.ThreadID(), len(gm.Data.Mentions))
	}
}

// Loại thread theo cmd như upstream (510 = 1-1, 511 = nhóm), không đoán theo payload: 511 rỗng vẫn là nhóm.
func TestOldMessagesLoaiThreadTheoCmd(t *testing.T) {
	ln := newRouterTestListener(t)
	ln.router(context.Background(), 1, 511, 1, plainFrame(`{"groupMsgs":[]}`))
	if old := recvOne(t, ln.OldMessages()); old.ThreadType != model.ThreadTypeGroup || len(old.Messages) != 0 {
		t.Fatalf("OldMessages = %+v, mong nhóm rỗng", old)
	}
	ln.router(context.Background(), 1, 611, 1, plainFrame(`{"reactGroups":[]}`))
	if old := recvOne(t, ln.OldReactions()); old.ThreadType != model.ThreadTypeGroup {
		t.Fatalf("OldReactions = %+v, mong nhóm", old)
	}
}

// Upstream PR #391 — thu hồi nằm trong lô tin cũ (phát lại sau khi nối lại) phải thành Undo,
// không lẫn vào OldMessages như một tin thường.
func TestOldMessagesThuHoiThanhUndo(t *testing.T) {
	t.Run("1-1", func(t *testing.T) {
		ln := newRouterTestListener(t)
		ln.selfListen = true
		ln.router(context.Background(), 1, 510, 1, plainFrame(`{"msgs":[`+oldText1+`,`+oldRecall1+`,`+oldText2+`]}`))

		old := recvOne(t, ln.OldMessages())
		if ids := oldMsgIDs(t, old); old.ThreadType != model.ThreadTypeUser || !slices.Equal(ids, []string{"1", "2"}) {
			t.Fatalf("OldMessages type %v ids %v, mong user [1 2]", old.ThreadType, ids)
		}
		u := recvOne(t, ln.Undo())
		if u.ThreadID != "200" || u.IsGroup || u.IsSelf || u.Data.Content.GlobalMsgID != 1 {
			t.Fatalf("Undo = %+v", u)
		}
	})
	t.Run("nhóm", func(t *testing.T) {
		ln := newRouterTestListener(t)
		ln.selfListen = true
		ln.router(context.Background(), 1, 511, 1, plainFrame(`{"groupMsgs":[
			{"msgId":"3","msgType":"webchat","uidFrom":"200","idTo":"300","content":"hello"},
			{"msgId":"r3","msgType":"chat.undo","uidFrom":"200","idTo":"300","content":{"globalMsgId":"3","cliMsgId":"1761089990003","deleteMsg":0}}]}`))

		old := recvOne(t, ln.OldMessages())
		if ids := oldMsgIDs(t, old); old.ThreadType != model.ThreadTypeGroup || !slices.Equal(ids, []string{"3"}) {
			t.Fatalf("OldMessages type %v ids %v, mong nhóm [3]", old.ThreadType, ids)
		}
		if u := recvOne(t, ln.Undo()); u.ThreadID != "300" || !u.IsGroup || u.Data.Content.GlobalMsgID != 3 {
			t.Fatalf("Undo = %+v", u)
		}
	})
	t.Run("tự thu hồi theo selfListen", func(t *testing.T) {
		own := `{"msgs":[{"msgId":"r4","msgType":"chat.undo","uidFrom":"0","idTo":"200","content":{"globalMsgId":"4","deleteMsg":0}}]}`
		for _, self := range []bool{true, false} {
			ln := newRouterTestListener(t)
			ln.selfListen = self
			ln.router(context.Background(), 1, 510, 1, plainFrame(own))
			if old := recvOne(t, ln.OldMessages()); len(old.Messages) != 0 {
				t.Fatalf("selfListen=%v: OldMessages = %d tin, mong 0", self, len(old.Messages))
			}
			if !self {
				assertEmpty(t, ln.Undo())
				continue
			}
			if u := recvOne(t, ln.Undo()); !u.IsSelf || u.ThreadID != "200" {
				t.Fatalf("Undo = %+v, mong tự thu hồi trên thread 200", u)
			}
		}
	})
}
