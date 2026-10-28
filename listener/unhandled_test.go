package listener

import (
	"context"
	"testing"
)

// cmd/subCmd không có trong bảng định tuyến phải ĐẾM được và lộ ra được, để ứng dụng biết
// mình đang bỏ sót gì (551/504/524/pinned_convers từng rơi đúng kiểu này).
func TestUnhandledCmdDemVaLoKhung(t *testing.T) {
	ln := newRouterTestListener(t)
	before := UnhandledFrames()

	ln.router(context.Background(), 1, 999, 0, plainFrame(`{"x":1}`))
	ln.router(context.Background(), 1, 501, 1, plainFrame(`{"msgs":[]}`)) // cmd biết, subCmd lạ

	if got := UnhandledFrames() - before; got != 2 {
		t.Fatalf("UnhandledFrames tăng %d, mong 2", got)
	}
	f := recvOne(t, ln.Unhandled())
	if f.Version != 1 || f.CMD != 999 || f.SubCMD != 0 || string(f.Data) != `{"x":1}` {
		t.Fatalf("frame = %+v (Data=%s)", f, f.Data)
	}
	if f = recvOne(t, ln.Unhandled()); f.CMD != 501 || f.SubCMD != 1 {
		t.Fatalf("frame = %+v", f)
	}
}

func TestUnhandledControlActType(t *testing.T) {
	ln := newRouterTestListener(t)
	before := UnhandledFrames()
	ln.router(context.Background(), 1, 601, 0, controlFrame(`{"act_type":"la_hoac","act":"x","data":{"k":"v"}}`))

	if got := UnhandledFrames() - before; got != 1 {
		t.Fatalf("UnhandledFrames tăng %d, mong 1", got)
	}
	f := recvOne(t, ln.Unhandled())
	if f.CMD != 601 || f.ActType != "la_hoac" || string(f.Data) != `{"k":"v"}` {
		t.Fatalf("frame = %+v (Data=%s)", f, f.Data)
	}
}

// Kênh Unhandled là kênh chẩn đoán tuỳ chọn: không ai đọc thì không được đẩy DroppedEvents lên
// (báo oan "mất tin" cho ứng dụng cũ không biết kênh này).
func TestUnhandledKhongDayDroppedEvents(t *testing.T) {
	ln := newRouterTestListener(t)
	dropped, unhandled := DroppedEvents(), UnhandledFrames()
	for range 200 {
		ln.router(context.Background(), 1, 999, 0, plainFrame(`{}`))
	}
	if got := DroppedEvents() - dropped; got != 0 {
		t.Fatalf("DroppedEvents tăng %d, mong 0", got)
	}
	if got := UnhandledFrames() - unhandled; got != 200 {
		t.Fatalf("UnhandledFrames tăng %d, mong 200", got)
	}
}

// Hành động bạn bè lạ không phải lỗi giải mã: lộ qua Unhandled (kèm act). "req" đi kèm "req_v2"
// và bị bỏ có chủ đích — không được thành nhiễu.
func TestUnhandledFriendActionLa(t *testing.T) {
	ln := newRouterTestListener(t)
	errsBefore, unhandledBefore := DecodeFailures(), UnhandledFrames()
	ln.router(context.Background(), 1, 601, 0, controlFrame(
		`{"act_type":"fr","act":"zzz_moi","data":{"k":1}}`,
		`{"act_type":"fr","act":"req","data":{"k":2}}`,
	))
	if got := DecodeFailures() - errsBefore; got != 0 {
		t.Fatalf("DecodeFailures tăng %d, mong 0", got)
	}
	if got := UnhandledFrames() - unhandledBefore; got != 1 {
		t.Fatalf("UnhandledFrames tăng %d, mong 1 (chỉ zzz_moi)", got)
	}
	if f := recvOne(t, ln.Unhandled()); f.ActType != "fr" || f.Act != "zzz_moi" || string(f.Data) != `{"k":1}` {
		t.Fatalf("frame = %+v (Data=%s)", f, f.Data)
	}
	assertEmpty(t, ln.Error())
}
