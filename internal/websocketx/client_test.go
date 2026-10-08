package websocketx_test

// Kiểm chứng wrapper truyền ForceDeflate xuống bản vendor (internal/coderws)
// và vòng connect → đọc message nén RSV1 → server đóng → reconnect chạy sạch,
// không panic — đúng kịch bản listener Zalo gặp ngoài production.

import (
	"bufio"
	"bytes"
	"compress/flate"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/luannt2002/zalo-api-unofficial/internal/websocketx"
)

// startZaloLikeWS: mỗi kết nối trả 101 KHÔNG echo extension, gửi 1 message
// TEXT nén permessage-deflate (RSV1) rồi close frame 1000 — như edge Zalo.
func startZaloLikeWS(t *testing.T, payload []byte) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Sec-WebSocket-Key")
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Error("khong hijack duoc")
			return
		}
		conn, brw, err := hj.Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		defer conn.Close()

		h := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
		_, _ = brw.WriteString("HTTP/1.1 101 Switching Protocols\r\n" +
			"Upgrade: websocket\r\n" +
			"Connection: Upgrade\r\n" +
			"Sec-WebSocket-Accept: " + base64.StdEncoding.EncodeToString(h[:]) + "\r\n\r\n")
		if err := brw.Flush(); err != nil {
			return
		}

		// Nén permessage-deflate, bỏ đuôi sync-flush 0x00 0x00 0xff 0xff.
		var buf bytes.Buffer
		fw, _ := flate.NewWriter(&buf, flate.BestSpeed)
		_, _ = fw.Write(payload)
		_ = fw.Flush()
		compressed := buf.Bytes()[:buf.Len()-4]

		// Frame TEXT, FIN=1, RSV1=1 (payload nhỏ, length 1 byte là đủ).
		frame := append([]byte{0x80 | 0x40 | 0x1, byte(len(compressed))}, compressed...)
		if _, err := conn.Write(frame); err != nil {
			return
		}
		// Close frame 1000.
		_, _ = conn.Write([]byte{0x80 | 0x8, 0x02, 0x03, 0xE8})
		_, _ = io.Copy(io.Discard, bufio.NewReader(conn))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestDialForceDeflateThenReconnect: 2 vòng connect liên tiếp (reconnect) đều
// phải đọc đúng message nén và nhận Closed 1000 sạch sẽ, không panic.
func TestDialForceDeflateThenReconnect(t *testing.T) {
	payload := []byte(`{"event":"message","data":"event Zalo nen RSV1 khong echo extension"}`)
	srv := startZaloLikeWS(t, payload)
	url := "ws" + strings.TrimPrefix(srv.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	for attempt := 0; attempt < 2; attempt++ {
		cl, err := websocketx.Dial(ctx, url, &websocketx.Options{
			ForceDeflate: true,
			Logf:         t.Logf,
		})
		if err != nil {
			t.Fatalf("lan %d: dial: %v", attempt, err)
		}

		select {
		case msg := <-cl.Messages():
			if msg.Type != websocketx.TextMessage {
				t.Fatalf("lan %d: type = %v, mong TextMessage", attempt, msg.Type)
			}
			if !bytes.Equal(msg.Data, payload) {
				t.Fatalf("lan %d: data = %q, mong %q", attempt, msg.Data, payload)
			}
		case <-ctx.Done():
			t.Fatalf("lan %d: khong nhan duoc message (listener chet nhu truoc khi va?)", attempt)
		}

		select {
		case ci := <-cl.Closed():
			if ci.Code != 1000 {
				t.Fatalf("lan %d: close code = %d (reason=%q err=%v), mong 1000", attempt, ci.Code, ci.Reason, ci.Err)
			}
		case <-ctx.Done():
			t.Fatalf("lan %d: khong nhan duoc Closed", attempt)
		}
	}
}

// TestDialWithoutForceDeflateStillFails — regression guard ở tầng wrapper:
// không bật ForceDeflate thì frame nén RSV1 phải gây lỗi "unexpected rsv bits"
// như production trước khi vá (sau đó stream lệch), KHÔNG được lặng lẽ đọc
// thành công. Nếu test này fail kiểu "đọc được message" nghĩa là semantics
// chuẩn RFC đã bị đổi ngoài chủ đích.
func TestDialWithoutForceDeflateStillFails(t *testing.T) {
	payload := []byte(`{"event":"message"}`)
	srv := startZaloLikeWS(t, payload)
	url := "ws" + strings.TrimPrefix(srv.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cl, err := websocketx.Dial(ctx, url, &websocketx.Options{Logf: t.Logf})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer cl.Close(1000, "test done")

	select {
	case msg := <-cl.Messages():
		t.Fatalf("doc duoc message %q du khong ForceDeflate — regression semantics RFC", msg.Data)
	case err := <-cl.Errors():
		if !strings.Contains(err.Error(), "rsv") {
			t.Fatalf("mong loi 'unexpected rsv bits' nhu production, got: %v", err)
		}
		t.Logf("loi nhu ky vong (hanh vi truoc khi va): %v", err)
	case ci := <-cl.Closed():
		// Một số đường chết kết thúc bằng close — cũng đạt, miễn không đọc được message.
		t.Logf("dong nhu ky vong: code=%d reason=%q err=%v", ci.Code, ci.Reason, ci.Err)
	case <-ctx.Done():
		t.Fatal("khong thay loi rsv lan connection dong")
	}
}
