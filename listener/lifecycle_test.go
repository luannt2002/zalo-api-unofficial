package listener

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	websocket "github.com/luannt2002/zalo-api-unofficial/internal/coderws"
	"github.com/luannt2002/zalo-api-unofficial/session"
)

// Máy chủ WebSocket giả: mỗi kết nối gọi handle với số thứ tự kết nối (từ 1).
type fakeWS struct {
	url   string
	conns atomic.Int32
}

func startFakeWS(t *testing.T, handle func(ctx context.Context, c *websocket.Conn, n int32)) *fakeWS {
	t.Helper()
	f := &fakeWS{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		handle(ctx, c, f.conns.Add(1))
	}))
	t.Cleanup(srv.Close)
	f.url = "ws" + strings.TrimPrefix(srv.URL, "http")
	return f
}

// keepOpen giữ kết nối tới khi client đóng.
func keepOpen(ctx context.Context, c *websocket.Conn) {
	for {
		if _, _, err := c.Read(ctx); err != nil {
			return
		}
	}
}

// sendCipherKey gửi khung cmd 1 subCmd 1 mang khoá — dấu hiệu "kết nối khoẻ" của Zalo.
func sendCipherKey(ctx context.Context, c *websocket.Conn) error {
	frame := append([]byte{1, 1, 0, 1}, []byte(`{"key":"a2V5"}`)...)
	return c.Write(ctx, websocket.MessageBinary, frame)
}

func retrySettings(code string, max int, times ...int) *session.Settings {
	s := &session.Settings{}
	s.Features.Socket.Retries = map[string]session.SocketRetryConfig{
		code: {Max: &max, Times: session.OneOrMany[int]{Values: times}},
	}
	return s
}

func newLiveListener(t *testing.T, url string, settings *session.Settings) *listener {
	t.Helper()
	sc := session.NewContext(session.WithLogging(false))
	sc.SealLogin(session.Seal{UID: testSelfUID, UserAgent: "test-ua", Settings: settings})
	ln, err := New(sc, []string{url})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(ln.Stop)
	return ln
}

func waitFor(t *testing.T, d time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("hết %v mà chưa: %s", d, what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func recvWithin[T any](t *testing.T, ch <-chan T, d time.Duration, what string) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(d):
		var zero T
		t.Fatalf("hết %v mà chưa nhận: %s", d, what)
		return zero
	}
}

// Ca 1 — Stop() trong lúc chờ thử lại phải huỷ lần nối lại. Bản cũ: client đang nil nên Stop()
// không làm gì, hẹn giờ vẫn nối lại một listener không ai quản (listener "ma").
func TestLifecycleStopHuyHenGioThuLai(t *testing.T) {
	f := startFakeWS(t, func(ctx context.Context, c *websocket.Conn, n int32) {
		if n == 1 {
			c.Close(4000, "bye")
			return
		}
		keepOpen(ctx, c)
	})
	ln := newLiveListener(t, f.url, retrySettings("4000", 5, 300))

	if err := ln.Start(context.Background(), true); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if ci := recvWithin(t, ln.Disconnected(), 5*time.Second, "Disconnected"); ci.Code != 4000 {
		t.Fatalf("Disconnected code = %d, mong 4000", ci.Code)
	}
	ln.Stop()

	time.Sleep(800 * time.Millisecond) // > trễ thử lại tối đa 300ms
	if n := f.conns.Load(); n != 1 {
		t.Fatalf("máy chủ nhận %d kết nối sau Stop(), mong 1 (không được nối lại)", n)
	}
}

// Ca 2 — Stop() rồi Start() lại trong lúc chờ: hẹn giờ của thế hệ cũ không được đụng vào kết nối
// mới (bản cũ: hẹn giờ cũ gọi Start → "Already started" → phát Closed giả trong khi đang sống).
func TestLifecycleKhoiDongLaiTrongLucCho(t *testing.T) {
	f := startFakeWS(t, func(ctx context.Context, c *websocket.Conn, n int32) {
		if n == 1 {
			c.Close(4000, "bye")
			return
		}
		keepOpen(ctx, c)
	})
	ln := newLiveListener(t, f.url, retrySettings("4000", 5, 300))

	if err := ln.Start(context.Background(), true); err != nil {
		t.Fatalf("Start: %v", err)
	}
	recvWithin(t, ln.Disconnected(), 5*time.Second, "Disconnected")
	ln.Stop()
	if err := ln.Start(context.Background(), true); err != nil {
		t.Fatalf("Start lại: %v", err)
	}

	time.Sleep(800 * time.Millisecond)
	select {
	case ci := <-ln.Closed():
		t.Fatalf("Closed %+v từ hẹn giờ thế hệ cũ trong khi kết nối mới đang sống", ci)
	default:
	}
	if n := f.conns.Load(); n != 2 {
		t.Fatalf("máy chủ nhận %d kết nối, mong 2", n)
	}
}

// Ca 3 — socket mở, nhận khoá rồi đóng ngay (phiên hỏng) KHÔNG được tính là nối thành công.
// Bản cũ xoá bộ đếm thử lại ngay khi nhận khoá ⇒ thử lại vô hạn, không bao giờ phát Closed.
func TestLifecycleSocketChapChonPhaiHetLuot(t *testing.T) {
	f := startFakeWS(t, func(ctx context.Context, c *websocket.Conn, n int32) {
		if sendCipherKey(ctx, c) != nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
		c.Close(4000, "flap")
	})
	ln := newLiveListener(t, f.url, retrySettings("4000", 2, 20))

	if err := ln.Start(context.Background(), true); err != nil {
		t.Fatalf("Start: %v", err)
	}
	ci := recvWithin(t, ln.Closed(), 5*time.Second, "Closed sau khi hết 2 lượt thử lại")
	if ci.Code != 4000 {
		t.Fatalf("Closed code = %d, mong 4000", ci.Code)
	}
	if n := f.conns.Load(); n != 3 {
		t.Fatalf("máy chủ nhận %d kết nối, mong 3 (1 + 2 lượt thử lại)", n)
	}
}

// Mặt còn lại của bảng chân trị: socket sống quá ngưỡng ổn định thì bộ đếm phải về 0, listener
// tiếp tục nối lại (không được phát Closed vì những lần đứt rời rạc).
func TestLifecycleSocketOnDinhThiXoaBoDem(t *testing.T) {
	f := startFakeWS(t, func(ctx context.Context, c *websocket.Conn, n int32) {
		if sendCipherKey(ctx, c) != nil {
			return
		}
		time.Sleep(300 * time.Millisecond)
		c.Close(4000, "rare")
	})
	ln := newLiveListener(t, f.url, retrySettings("4000", 1, 20))
	ln.stableAfter = 100 * time.Millisecond

	if err := ln.Start(context.Background(), true); err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitFor(t, 5*time.Second, "≥4 kết nối", func() bool { return f.conns.Load() >= 4 })
	select {
	case ci := <-ln.Closed():
		t.Fatalf("Closed %+v dù mỗi socket đều sống quá ngưỡng ổn định", ci)
	default:
	}
}
