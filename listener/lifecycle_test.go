package listener

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

// Ca 4 — mất TCP không có khung đóng là 1006 (RFC 6455 §7.1.5), không phải 1011. Bản cũ báo
// 1011 nên lịch thử lại dành cho 1006 không bao giờ chạy với lần đứt mạng thật.
func TestLifecycleMatTCPLa1006(t *testing.T) {
	f := startFakeWS(t, func(ctx context.Context, c *websocket.Conn, n int32) {
		c.CloseNow()
	})
	ln := newLiveListener(t, f.url, nil)

	if err := ln.Start(context.Background(), false); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if ci := recvWithin(t, ln.Disconnected(), 5*time.Second, "Disconnected"); ci.Code != ZaloAbnormalClosure {
		t.Fatalf("Disconnected code = %d, mong %d", ci.Code, ZaloAbnormalClosure)
	}
}

// Ca 5 — 1006 không có lịch từ máy chủ: giãn 2/5/10/20/30/60 s, tối đa 8 lần, rồi hết lượt để
// listener phát Closed cho ứng dụng đăng nhập lại (bản cũ thử lại vô hạn, Closed không bao giờ tới).
func TestLifecycle1006CoTran(t *testing.T) {
	ln := newLiveListener(t, "wss://primary.example", &session.Settings{})

	want := []int{2000, 5000, 10000, 20000, 30000, 60000, 60000, 60000}
	for i, d := range want {
		got, ok := ln.canRetry(ZaloAbnormalClosure)
		if !ok || got < d/2 || got > d {
			t.Fatalf("lần %d: canRetry(1006) = (%d, %v), mong trong [%d, %d]", i+1, got, ok, d/2, d)
		}
	}
	if got, ok := ln.canRetry(ZaloAbnormalClosure); ok {
		t.Fatalf("lần 9: canRetry(1006) = (%d, true), mong hết lượt", got)
	}
	ln.resetRetryCounters()
	if _, ok := ln.canRetry(ZaloAbnormalClosure); !ok {
		t.Fatal("sau reset phải thử lại được")
	}
}

// Máy chủ giả đếm MỌI lượt quay số (kể cả lượt bị từ chối); handle trả false = từ chối nâng cấp
// (503) ⇒ phía client là lỗi quay số, như mạng còn đứt.
func startDialCountingWS(t *testing.T, handle func(ctx context.Context, n int32, accept func() *websocket.Conn) bool) (string, *atomic.Int32) {
	t.Helper()
	var reqs atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		var c *websocket.Conn
		accept := func() *websocket.Conn {
			c, _ = websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
			return c
		}
		if !handle(ctx, reqs.Add(1), accept) {
			http.Error(w, "down", http.StatusServiceUnavailable)
		}
		if c != nil {
			c.CloseNow()
		}
	}))
	t.Cleanup(srv.Close)
	return "ws" + strings.TrimPrefix(srv.URL, "http"), &reqs
}

// P1-b — nối lại mà quay số lỗi (mạng chưa về) phải đi tiếp lịch thử lại, không phát Closed ngay.
// Kết nối 1 rớt (1006), lượt 2 và 3 bị từ chối, lượt 4 sống ⇒ phải nối lại được.
func TestLifecycleQuaySoLoiVanTheoLich(t *testing.T) {
	url, reqs := startDialCountingWS(t, func(ctx context.Context, n int32, accept func() *websocket.Conn) bool {
		switch n {
		case 2, 3:
			return false
		case 1:
			accept() // CloseNow ngay ⇒ client thấy 1006
		default:
			if c := accept(); c != nil {
				keepOpen(ctx, c)
			}
		}
		return true
	})
	ln := newLiveListener(t, url, retrySettings("1006", 5, 20))

	if err := ln.Start(context.Background(), true); err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitFor(t, 3*time.Second, "lượt quay số thứ 4 (nối lại sau 2 lần lỗi)", func() bool { return reqs.Load() >= 4 })
	time.Sleep(200 * time.Millisecond)
	select {
	case ci := <-ln.Closed():
		t.Fatalf("Closed %+v dù lượt 4 đã nối được", ci)
	default:
	}
	if n := reqs.Load(); n != 4 {
		t.Fatalf("máy chủ nhận %d lượt quay số, mong 4 (đứng yên khi đã nối lại)", n)
	}
	if ln.getClient() == nil {
		t.Fatal("không có kết nối sống sau khi máy chủ bật lại")
	}
}

// P1-b — máy chủ tắt hẳn: hết đúng số lượt của lịch (1 + 3) rồi mới Closed, mã 1006.
func TestLifecycleQuaySoLoiHetLuotMoiClosed(t *testing.T) {
	url, reqs := startDialCountingWS(t, func(ctx context.Context, n int32, accept func() *websocket.Conn) bool {
		if n == 1 {
			accept()
			return true
		}
		return false
	})
	ln := newLiveListener(t, url, retrySettings("1006", 3, 20))

	if err := ln.Start(context.Background(), true); err != nil {
		t.Fatalf("Start: %v", err)
	}
	ci := recvWithin(t, ln.Closed(), 3*time.Second, "Closed sau khi hết lượt")
	if ci.Code != ZaloAbnormalClosure || ci.Err == nil {
		t.Fatalf("Closed = %+v, mong 1006 kèm lỗi quay số", ci)
	}
	time.Sleep(100 * time.Millisecond)
	if n := reqs.Load(); n != 4 {
		t.Fatalf("máy chủ nhận %d lượt quay số, mong 4 (1 + 3 lượt thử lại)", n)
	}
}

// Stop() trên kết nối đang sống rồi Start() lại phải được (upstream: stop() → reset() → ws = null).
// Bản cũ: run thoát theo ctx mà không reset ⇒ client cũ còn đó ⇒ "Already started".
func TestLifecycleStopRoiStartLai(t *testing.T) {
	f := startFakeWS(t, func(ctx context.Context, c *websocket.Conn, n int32) { keepOpen(ctx, c) })
	ln := newLiveListener(t, f.url, nil)

	for i := range 5 {
		if err := ln.Start(context.Background(), true); err != nil {
			t.Fatalf("lần %d: Start: %v", i+1, err)
		}
		waitFor(t, 3*time.Second, "máy chủ nhận kết nối", func() bool { return f.conns.Load() == int32(i+1) })
		ln.Stop()
		if ln.getClient() != nil {
			t.Fatalf("lần %d: client vẫn còn sau Stop()", i+1)
		}
	}
}

// P1 (soát rc.2) — callback của lượt nối lại quay số LỖI không được đụng vào kết nối mà một Start()
// mới hơn đã dựng. Ca: mạng đứt, lượt quay số 2 lỗi; ứng dụng gọi Start() (hoặc Stop() rồi Start())
// ngay lúc đó. Bản cũ: callback cũ reset() / cancelActiveContext() không kiểm context của mình còn
// hiệu lực ⇒ xoá client mới, huỷ context mới ⇒ Start() trả nil mà listener đã chết.
//
// Thứ tự hỏng (Start giành ln.mu xen giữa lúc lượt cũ nhả khoá và lúc callback cũ reset) bình thường
// rất hẹp. Test ép nó tất định bằng chế độ đói của sync.Mutex: giữ ln.mu tới khi hẹn giờ và Start đều
// xếp hàng, nhả rồi giành lại ngay ⇒ hẹn giờ thức dậy giành hụt sau > 1 ms chờ ⇒ mutex chuyển chế độ
// đói ⇒ khoá được trao thẳng theo hàng đợi: hẹn giờ → (Stop) → Start, không goroutine nào chen ngang.
func TestLifecycleCallbackCuKhongGietStartMoi(t *testing.T) {
	for _, withStop := range []bool{false, true} {
		name := "Start"
		if withStop {
			name = "StopRoiStart"
		}
		t.Run(name, func(t *testing.T) { callbackCuKhongGietStartMoi(t, withStop) })
	}
}

func callbackCuKhongGietStartMoi(t *testing.T, withStop bool) {
	dialing, release := make(chan struct{}), make(chan struct{})
	var open atomic.Int32
	url, _ := startDialCountingWS(t, func(ctx context.Context, n int32, accept func() *websocket.Conn) bool {
		switch n {
		case 1:
			accept() // rớt ngay ⇒ 1006 ⇒ hẹn giờ nối lại
			return true
		case 2:
			close(dialing)
			<-release
			return false // lượt nối lại lỗi
		}
		if c := accept(); c != nil {
			open.Add(1)
			keepOpen(ctx, c)
			open.Add(-1)
		}
		return true
	})
	ln := newLiveListener(t, url, retrySettings("1006", 5, 300))
	// Chạy trước cleanup Stop()/máy chủ (LIFO): lượt quay số đang chờ release không được treo chúng.
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	if err := ln.Start(context.Background(), true); err != nil {
		t.Fatalf("Start: %v", err)
	}
	recvWithin(t, ln.Disconnected(), 3*time.Second, "Disconnected của kết nối 1")
	time.Sleep(30 * time.Millisecond) // canRetry + hẹn giờ đã xong

	// Chỉ TryLock: nếu hẹn giờ đã giữ ln.mu (nó chờ release, mà release do test đóng) thì Lock sẽ treo.
	// Không ép được thứ tự (máy quá tải) thì chạy tiếp không ép — điều kiện kiểm vẫn đúng cho code đúng.
	held := ln.mu.TryLock()
	if held {
		time.Sleep(400 * time.Millisecond) // > trễ tối đa 300 ms ⇒ hẹn giờ đang chờ ln.mu
	} else {
		recvWithin(t, dialing, 3*time.Second, "lượt nối lại thứ 2") // hẹn giờ đang quay số, giữ ln.mu
	}
	if withStop {
		go ln.Stop()
		time.Sleep(20 * time.Millisecond)
	}
	startErr := make(chan error, 1)
	go func() { startErr <- ln.Start(context.Background(), true) }()
	time.Sleep(20 * time.Millisecond)
	if held {
		ln.mu.Unlock()
		if ln.mu.TryLock() { // giành lại ⇒ hẹn giờ giành hụt ⇒ chế độ đói
			time.Sleep(10 * time.Millisecond)
			ln.mu.Unlock()
		}
	}

	recvWithin(t, dialing, 3*time.Second, "lượt nối lại thứ 2")
	unblock()
	if err := recvWithin(t, startErr, 3*time.Second, "Start() của ứng dụng"); err != nil {
		t.Fatalf("Start() của ứng dụng: %v", err)
	}

	time.Sleep(300 * time.Millisecond)
	if ln.getClient() == nil {
		t.Fatal("Start() trả nil nhưng client mới đã bị callback cũ xoá")
	}
	if n := open.Load(); n != 1 {
		t.Fatalf("máy chủ còn %d kết nối mở, mong 1 (context mới bị huỷ?)", n)
	}
	select {
	case ci := <-ln.Closed():
		t.Fatalf("Closed %+v trong khi kết nối mới đang sống", ci)
	default:
	}
}

// 7170fa4 — Stop() phải huỷ và đọc client trong MỘT vùng khoá. Bản cũ đọc client (RLock) rồi mới
// huỷ (Lock): một lượt nối lại giành khoá xen giữa ⇒ tạo client mà Stop không đóng, ln.client kẹt
// lại ⇒ Start() sau đó báo "Already started". Ép thứ tự bằng chế độ đói như test P1: hẹn giờ đứng
// đầu hàng đợi; khi nhả khoá, RLock của Stop cũ được đi trước, rồi khoá trao thẳng cho hẹn giờ.
func TestLifecycleStopKhongLotLuotNoiLai(t *testing.T) {
	var open atomic.Int32
	url, _ := startDialCountingWS(t, func(ctx context.Context, n int32, accept func() *websocket.Conn) bool {
		c := accept()
		if n == 1 || c == nil {
			return true // rớt ngay ⇒ 1006 ⇒ hẹn giờ nối lại
		}
		open.Add(1)
		keepOpen(ctx, c)
		open.Add(-1)
		return true
	})
	ln := newLiveListener(t, url, retrySettings("1006", 5, 300))
	if err := ln.Start(context.Background(), true); err != nil {
		t.Fatalf("Start: %v", err)
	}
	recvWithin(t, ln.Disconnected(), 3*time.Second, "Disconnected của kết nối 1")
	time.Sleep(30 * time.Millisecond)

	ln.mu.Lock()
	time.Sleep(400 * time.Millisecond) // hẹn giờ đang chờ ln.mu
	ln.mu.Unlock()
	ln.mu.Lock() // hẹn giờ giành hụt ⇒ chế độ đói, nó đứng đầu hàng
	time.Sleep(10 * time.Millisecond)
	stopped := make(chan struct{})
	go func() { ln.Stop(); close(stopped) }()
	time.Sleep(20 * time.Millisecond)
	ln.mu.Unlock()
	recvWithin(t, stopped, 5*time.Second, "Stop() trả về")

	waitFor(t, 3*time.Second, "kết nối của lượt nối lại bị đóng", func() bool { return open.Load() == 0 })
	if ln.getClient() != nil {
		t.Fatal("client của lượt nối lại còn kẹt sau Stop()")
	}
	if err := ln.Start(context.Background(), true); err != nil {
		t.Fatalf("Start() sau Stop(): %v", err)
	}
}
