package events

import (
	"testing"

	"github.com/luannt2002/zalo-api-unofficial/internal/jsonx"
	"github.com/luannt2002/zalo-api-unofficial/model"
)

const (
	textItem = `{"msgId":"m1","cliMsgId":"c1","msgType":"webchat","uidFrom":"200","idTo":"0","ts":"1761624780000","status":1,"content":"xin chào"}`
	undoItem = `{"msgId":"r1","cliMsgId":"rc1","msgType":"chat.undo","uidFrom":"200","idTo":"0","ts":"1761624790000","content":{"globalMsgId":"7300000000001","cliMsgId":"1761624700000","deleteMsg":0,"srcId":"0","destId":"0"}}`
)

// Tin thường (đa số khung) không được trả giá giải TUndo thất bại qua đường khoan dung (cây JSON +
// marshal lại). Đo trên go1.25: giải thẳng TMessage 14 alloc; thử TUndo trước 94; đọc trước msgType 20–21.
// Ngưỡng < 2× giải thẳng: chặn được bản cũ, chịu được chênh lệch giữa các bản Go.
func TestTinThuongKhongThuGiaiUndo(t *testing.T) {
	item := []byte(textItem)
	direct := testing.AllocsPerRun(200, func() {
		var tm model.TMessage
		_ = jsonx.UnmarshalLenient(item, &tm)
	})
	for name, dec := range map[string]func(){
		"501": func() { var m messageOrUndo; _ = m.UnmarshalJSON(item) },
		"521": func() { var m groupMessageOrUndo; _ = m.UnmarshalJSON(item) },
	} {
		if got := testing.AllocsPerRun(200, dec); got >= 2*direct {
			t.Errorf("%s: %.0f alloc/item, giải thẳng TMessage %.0f — đang giải thừa", name, got, direct)
		}
	}
}

// Ranh giới tin/thu hồi giữ nguyên: msgType chat.undo ⇒ Undo, còn lại ⇒ Message.
func TestPhanBietTinVaThuHoi(t *testing.T) {
	var m messageOrUndo
	if err := m.UnmarshalJSON([]byte(textItem)); err != nil || m.Message == nil || m.Undo != nil || m.Message.MsgID != "m1" {
		t.Fatalf("tin thường = %+v (err %v)", m, err)
	}
	var u messageOrUndo
	if err := u.UnmarshalJSON([]byte(undoItem)); err != nil || u.Undo == nil || u.Message != nil || u.Undo.Content.GlobalMsgID != 7300000000001 {
		t.Fatalf("thu hồi = %+v (err %v)", u, err)
	}
	var g groupMessageOrUndo
	if err := g.UnmarshalJSON([]byte(undoItem)); err != nil || g.Undo == nil || g.Message != nil {
		t.Fatalf("thu hồi nhóm = %+v (err %v)", g, err)
	}
}

func BenchmarkMessageOrUndoText(b *testing.B) {
	item := []byte(textItem)
	b.ReportAllocs()
	for b.Loop() {
		var m messageOrUndo
		_ = m.UnmarshalJSON(item)
	}
}
