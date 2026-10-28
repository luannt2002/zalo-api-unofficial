package listener

import (
	"context"
	"errors"
	"testing"

	"github.com/luannt2002/zalo-api-unofficial/model"
)

func actionsFrame(items ...string) BaseWSMessage {
	s := ""
	for i, it := range items {
		if i > 0 {
			s += ","
		}
		s += it
	}
	return plainFrame(`{"actions":[` + s + `]}`)
}

// 5(b) — typing đi cùng cơ chế khoan dung: ts dạng số, isPC dạng chuỗi vẫn phải giải ra đúng người gõ.
// Bản cũ giải cứng ⇒ lỗi kiểu bị nuốt ⇒ phát typing rỗng (UID "").
func TestTypingKhoanDung(t *testing.T) {
	ln := newRouterTestListener(t)
	ln.router(context.Background(), 1, 602, 0, actionsFrame(
		`{"act_type":"typing","act":"typing","data":"\"uid\":\"200\",\"ts\":1761624780000,\"isPC\":\"1\""}`,
		`{"act_type":"typing","act":"gtyping","data":"{\"uid\":\"201\",\"gid\":900,\"ts\":\"1761624780001\",\"isPC\":0}"}`,
	))

	ut, ok := recvOne(t, ln.Typing()).(model.UserTyping)
	if !ok || ut.ThreadID() != "200" || ut.Data.TS != "1761624780000" || ut.Data.IsPC != 1 {
		t.Fatalf("UserTyping = %+v", ut)
	}
	gt, ok := recvOne(t, ln.Typing()).(model.GroupTyping)
	if !ok || gt.ThreadID() != "900" || gt.Data.UID != "201" {
		t.Fatalf("GroupTyping = %+v", gt)
	}
	assertEmpty(t, ln.Error())
}

// Typing không giải được: báo *DecodeError + đếm, không phát typing rỗng.
func TestTypingHongBaoLoi(t *testing.T) {
	ln := newRouterTestListener(t)
	before := DecodeFailures()
	ln.router(context.Background(), 1, 602, 0, actionsFrame(`{"act_type":"typing","act":"typing","data":{"uid":{"x":1}}}`))

	assertEmpty(t, ln.Typing())
	var de *DecodeError
	if err := recvOne(t, ln.Error()); !errors.As(err, &de) || de.Op != "listener.handleActions" {
		t.Fatalf("err = %v, mong *DecodeError từ listener.handleActions", err)
	}
	if got := DecodeFailures() - before; got != 1 {
		t.Fatalf("DecodeFailures tăng %d, mong 1", got)
	}
}

// Hành động 602 không phải typing trước đây bị bỏ im lặng; giờ lộ qua Unhandled như control 601 lạ.
func TestActionLaVaoUnhandled(t *testing.T) {
	ln := newRouterTestListener(t)
	before := UnhandledFrames()
	ln.router(context.Background(), 1, 602, 0, actionsFrame(
		`{"act_type":"la_hoac","act":"x","data":"{\"k\":1}"}`,
		`{"act_type":"typing","act":"typing_moi","data":"\"uid\":\"200\""}`,
	))
	if got := UnhandledFrames() - before; got != 2 {
		t.Fatalf("UnhandledFrames tăng %d, mong 2", got)
	}
	if f := recvOne(t, ln.Unhandled()); f.CMD != 602 || f.ActType != "la_hoac" || f.Act != "x" || string(f.Data) != `{"k":1}` {
		t.Fatalf("frame = %+v (Data=%s)", f, f.Data)
	}
	if f := recvOne(t, ln.Unhandled()); f.ActType != "typing" || f.Act != "typing_moi" {
		t.Fatalf("frame = %+v", f)
	}
	assertEmpty(t, ln.Typing())
}
