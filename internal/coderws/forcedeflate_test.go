//go:build !js

package websocket_test

// Test cho 2 PATCH FORK (Zalo) trong bản vendor — xem doc.go:
//  1. Offer extension giống hệt Chrome: "permessage-deflate; client_max_window_bits".
//  2. ForceDeflate: server KHÔNG echo extension nhưng vẫn gửi frame nén RSV1
//     (hành vi thật của edge WS Zalo, đã xác nhận bằng production log) — client
//     phải giải nén message RSV1, đọc raw message không RSV1, và KHÔNG nén
//     chiều gửi.

import (
	"bufio"
	"bytes"
	"compress/flate"
	"context"
	crand "crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	websocket "github.com/luannt2002/zalo-api-unofficial/internal/coderws"
)

const (
	opcodeContinuation = 0x0
	opcodeText         = 0x1
	opcodeClose        = 0x8
	opcodePong         = 0xa

	// Format offer của Chrome — server Zalo phân nhánh theo đúng chuỗi này.
	chromeExtensionOffer = "permessage-deflate; client_max_window_bits"
)

// fakeZaloServer mô phỏng hành vi đã quan sát trên production của edge WS Zalo:
// handshake 101 OK nhưng KHÔNG echo Sec-WebSocket-Extensions, sau đó vẫn gửi
// frame nén permessage-deflate (RSV1) vô điều kiện xen kẽ frame không nén.
type fakeZaloServer struct {
	srv *httptest.Server

	mu     sync.Mutex
	offers []string // Sec-WebSocket-Extensions client offer ở mỗi lần handshake
}

func (f *fakeZaloServer) wsURL() string {
	return "ws" + strings.TrimPrefix(f.srv.URL, "http")
}

func (f *fakeZaloServer) firstOffer() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.offers) == 0 {
		return ""
	}
	return f.offers[0]
}

// startFakeZaloServer chạy script trên từng kết nối SAU khi đã trả 101 không
// kèm Sec-WebSocket-Extensions (điểm mấu chốt tái hiện lỗi production).
func startFakeZaloServer(t *testing.T, script func(t *testing.T, conn net.Conn, br *bufio.Reader)) *fakeZaloServer {
	t.Helper()

	f := &fakeZaloServer{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.offers = append(f.offers, r.Header.Get("Sec-WebSocket-Extensions"))
		f.mu.Unlock()

		key := r.Header.Get("Sec-WebSocket-Key")

		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Error("response writer khong ho tro hijack")
			return
		}
		conn, brw, err := hj.Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		defer conn.Close()

		// 101 thiếu Sec-WebSocket-Extensions dù client đã offer — như server Zalo.
		accept := secWebSocketAccept(key)
		_, _ = brw.WriteString("HTTP/1.1 101 Switching Protocols\r\n" +
			"Upgrade: websocket\r\n" +
			"Connection: Upgrade\r\n" +
			"Sec-WebSocket-Accept: " + accept + "\r\n\r\n")
		if err := brw.Flush(); err != nil {
			t.Errorf("flush handshake: %v", err)
			return
		}

		script(t, conn, brw.Reader)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func secWebSocketAccept(key string) string {
	h := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(h[:])
}

// writeServerFrame ghi 1 frame server→client (không mask) ở mức byte thô.
func writeServerFrame(w io.Writer, fin, rsv1 bool, opcode byte, payload []byte) error {
	b0 := opcode
	if fin {
		b0 |= 0x80
	}
	if rsv1 {
		b0 |= 0x40
	}

	hdr := []byte{b0}
	switch n := len(payload); {
	case n < 126:
		hdr = append(hdr, byte(n))
	case n < 1<<16:
		hdr = append(hdr, 126, byte(n>>8), byte(n))
	default:
		var l [8]byte
		binary.BigEndian.PutUint64(l[:], uint64(n))
		hdr = append(append(hdr, 127), l[:]...)
	}

	if _, err := w.Write(hdr); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

// pmceCompressor nén message theo permessage-deflate với context takeover
// (GIỮ flate.Writer giữa các message — đúng cách server Zalo nén cho browser).
type pmceCompressor struct {
	buf bytes.Buffer
	fw  *flate.Writer
}

func newPMCECompressor(t *testing.T) *pmceCompressor {
	t.Helper()
	c := &pmceCompressor{}
	fw, err := flate.NewWriter(&c.buf, flate.BestSpeed)
	if err != nil {
		t.Fatalf("flate.NewWriter: %v", err)
	}
	c.fw = fw
	return c
}

func (c *pmceCompressor) compress(t *testing.T, p []byte) []byte {
	t.Helper()
	c.buf.Reset()
	if _, err := c.fw.Write(p); err != nil {
		t.Fatalf("flate write: %v", err)
	}
	if err := c.fw.Flush(); err != nil {
		t.Fatalf("flate flush: %v", err)
	}
	out := append([]byte(nil), c.buf.Bytes()...)
	// RFC 7692: bỏ đuôi sync-flush 0x00 0x00 0xff 0xff trước khi đóng frame.
	if !bytes.HasSuffix(out, []byte("\x00\x00\xff\xff")) {
		t.Fatalf("flate flush khong ket thuc bang sync marker: %x", out)
	}
	return out[:len(out)-4]
}

// readClientFrame đọc 1 frame client→server (đã mask) và trả về byte đầu +
// payload đã unmask — dùng kiểm chứng chiều gửi không nén (RSV1 phải = 0).
func readClientFrame(br *bufio.Reader) (b0 byte, payload []byte, err error) {
	var hdr [2]byte
	if _, err = io.ReadFull(br, hdr[:]); err != nil {
		return 0, nil, err
	}
	b0 = hdr[0]

	n := int64(hdr[1] & 0x7f)
	masked := hdr[1]&0x80 != 0
	switch n {
	case 126:
		var ext [2]byte
		if _, err = io.ReadFull(br, ext[:]); err != nil {
			return 0, nil, err
		}
		n = int64(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		if _, err = io.ReadFull(br, ext[:]); err != nil {
			return 0, nil, err
		}
		n = int64(binary.BigEndian.Uint64(ext[:]))
	}

	var maskKey [4]byte
	if masked {
		if _, err = io.ReadFull(br, maskKey[:]); err != nil {
			return 0, nil, err
		}
	}

	payload = make([]byte, n)
	if _, err = io.ReadFull(br, payload); err != nil {
		return 0, nil, err
	}
	if masked {
		for i := range payload {
			payload[i] ^= maskKey[i%4]
		}
	}
	return b0, payload, nil
}

// TestForceDeflateReadsMixedCompressedAndRawMessages — ca quan trọng nhất:
// client ForceDeflate phải đọc đúng cả message nén (RSV1) lẫn message raw
// xen kẽ, giữ context takeover giữa các message nén, và gửi đi KHÔNG nén.
func TestForceDeflateReadsMixedCompressedAndRawMessages(t *testing.T) {
	t.Parallel()

	msgs := [][]byte{
		[]byte(`{"event":"message","data":"xin chao tu server nen permessage-deflate khong echo extension"}`),
		[]byte(`{"event":"raw","data":"frame khong nen, khong RSV1, phai doc nguyen van"}`),
		[]byte(`{"event":"message","data":"xin chao tu server nen permessage-deflate khong echo extension lan hai"}`),
	}
	clientPayload := bytes.Repeat([]byte("zalo-listener-gui-len-khong-nen-"), 32) // > flateThreshold 128

	serverDone := make(chan struct{})
	f := startFakeZaloServer(t, func(t *testing.T, conn net.Conn, br *bufio.Reader) {
		defer close(serverDone)
		comp := newPMCECompressor(t)

		// msg 1: nén, RSV1=1 — đúng pha gây "unexpected rsv bits" khi chưa vá.
		if err := writeServerFrame(conn, true, true, opcodeText, comp.compress(t, msgs[0])); err != nil {
			t.Errorf("ghi msg 0: %v", err)
			return
		}
		// msg 2: raw, RSV1=0 — semantics per-message phải giữ chuẩn.
		if err := writeServerFrame(conn, true, false, opcodeText, msgs[1]); err != nil {
			t.Errorf("ghi msg 1: %v", err)
			return
		}
		// msg 3: nén tiếp bằng CÙNG flate.Writer — kiểm chứng context takeover.
		if err := writeServerFrame(conn, true, true, opcodeText, comp.compress(t, msgs[2])); err != nil {
			t.Errorf("ghi msg 2: %v", err)
			return
		}

		// Chiều gửi: frame từ client phải KHÔNG nén (RSV1=0, payload nguyên văn).
		b0, payload, err := readClientFrame(br)
		if err != nil {
			t.Errorf("doc frame client: %v", err)
			return
		}
		if b0&0x40 != 0 {
			t.Error("client dat RSV1 khi gui du flateWriteDisabled — chieu gui phai khong nen")
		}
		if b0&0x0f != opcodeText {
			t.Errorf("opcode client = %#x, mong opText", b0&0x0f)
		}
		if !bytes.Equal(payload, clientPayload) {
			t.Errorf("payload client bi bien dang (len=%d, mong %d) — co the da bi nen", len(payload), len(clientPayload))
		}

		_, _ = io.Copy(io.Discard, br) // giữ kết nối tới khi client đóng
	})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	c, resp, err := websocket.Dial(ctx, f.wsURL(), &websocket.DialOptions{
		CompressionMode: websocket.CompressionContextTakeover,
		ForceDeflate:    true,
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.CloseNow()

	if got := resp.Header.Get("Sec-WebSocket-Extensions"); got != "" {
		t.Fatalf("server gia lap phai tra ext rong nhu Zalo that, got %q", got)
	}
	// Patch 1: offer phải đúng nguyên văn format Chrome.
	if offer := f.firstOffer(); offer != chromeExtensionOffer {
		t.Fatalf("offer extension = %q, mong %q", offer, chromeExtensionOffer)
	}

	for i, want := range msgs {
		typ, data, err := c.Read(ctx)
		if err != nil {
			t.Fatalf("doc msg %d: %v", i, err)
		}
		if typ != websocket.MessageText {
			t.Fatalf("msg %d type = %v, mong MessageText", i, typ)
		}
		if !bytes.Equal(data, want) {
			t.Fatalf("msg %d = %q, mong %q", i, data, want)
		}
	}

	if err := c.Write(ctx, websocket.MessageText, clientPayload); err != nil {
		t.Fatalf("ghi nguoc len server: %v", err)
	}

	c.CloseNow()
	select {
	case <-serverDone:
	case <-ctx.Done():
		t.Fatal("server script khong ket thuc")
	}
}

// TestWithoutForceDeflateCompressedFrameStillFails — regression guard: KHÔNG
// bật ForceDeflate thì frame nén RSV1 từ server không-echo-extension phải lỗi
// đúng như production trước khi vá ("unexpected rsv bits"). Nếu test này fail
// nghĩa là semantics chuẩn RFC của bản vendor đã bị đổi ngoài chủ đích.
func TestWithoutForceDeflateCompressedFrameStillFails(t *testing.T) {
	t.Parallel()

	f := startFakeZaloServer(t, func(t *testing.T, conn net.Conn, br *bufio.Reader) {
		comp := newPMCECompressor(t)
		if err := writeServerFrame(conn, true, true, opcodeText, comp.compress(t, []byte("nen RSV1 vo dieu kien"))); err != nil {
			return
		}
		_, _ = io.Copy(io.Discard, br)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	c, _, err := websocket.Dial(ctx, f.wsURL(), &websocket.DialOptions{
		CompressionMode: websocket.CompressionContextTakeover,
		// ForceDeflate: false — hành vi upstream thuần.
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.CloseNow()

	_, _, err = c.Read(ctx)
	if err == nil {
		t.Fatal("mong loi rsv nhu truoc khi va, nhung doc thanh cong")
	}
	if !strings.Contains(err.Error(), "rsv") {
		t.Fatalf("mong loi 'unexpected rsv bits', got: %v", err)
	}
}

// fullDeflateStream nén p thành MỘT deflate stream TRỌN VẸN (Write + Close ⇒
// block cuối BFINAL=1, kiểu zlib Z_FINISH từng message) — KHÔNG sync-flush,
// KHÔNG strip đuôi 00 00 ff ff như RFC 7692 §7.2.1. Đây là kiểu nén khiến
// inflater phía client trả io.EOF NGAY khi hết block cuối, trước khi tiêu thụ
// hết phần còn lại của message WebSocket.
func fullDeflateStream(t *testing.T, p []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	fw, err := flate.NewWriter(&buf, flate.BestSpeed)
	if err != nil {
		t.Fatalf("flate.NewWriter: %v", err)
	}
	if _, err := fw.Write(p); err != nil {
		t.Fatalf("flate write: %v", err)
	}
	if err := fw.Close(); err != nil {
		t.Fatalf("flate close: %v", err)
	}
	return buf.Bytes()
}

// writeFragmentedServerMessage gửi payload nén chia thành nhiều frame: frame
// đầu opText|RSV1 fin=0, tiếp theo continuation, và nếu emptyFin thì kết thúc
// bằng FIN frame RỖNG — đúng kiểu sender stream (chính coder/websocket chiều
// ghi cũng kết thúc message bằng fin frame rỗng như vậy).
func writeFragmentedServerMessage(w io.Writer, payload []byte, chunk int, emptyFin bool) error {
	first := true
	for len(payload) > 0 {
		n := chunk
		if n > len(payload) {
			n = len(payload)
		}
		part := payload[:n]
		payload = payload[n:]

		op := byte(opcodeContinuation)
		rsv1 := false
		if first {
			op = opcodeText
			rsv1 = true
			first = false
		}
		fin := len(payload) == 0 && !emptyFin
		if err := writeServerFrame(w, fin, rsv1, op, part); err != nil {
			return err
		}
	}
	if emptyFin {
		return writeServerFrame(w, true, false, opcodeContinuation, nil)
	}
	return nil
}

// TestForceDeflateFullStreamMessages — tái hiện ĐÚNG lỗi production
// "failed to get reader: previous message not read to completion" (lặp vô hạn
// sau khi deploy patch ForceDeflate): server nén mỗi message thành MỘT deflate
// stream trọn vẹn (BFINAL=1) thay vì sync-flush + strip đuôi theo RFC 7692
// §7.2.1, kèm các biến thể thực tế:
//   - message lớn fragment nhiều frame + FIN frame RỖNG cuối — inflater trả
//     io.EOF ở cuối frame dữ liệu chót mà KHÔNG đọc FIN frame rỗng
//     ⇒ msgReader.fin kẹt false ⇒ mọi Read() sau lỗi vĩnh viễn;
//   - byte padding 0x00 sau block cuối (RFC 7692 §7.2.3.6 cho phép sender chèn);
//   - đuôi 00 00 ff ff KHÔNG strip nằm lại trong frame.
//
// Client ForceDeflate phải Read() lặp qua TẤT CẢ message không lỗi.
func TestForceDeflateFullStreamMessages(t *testing.T) {
	t.Parallel()

	// Payload lớn khó nén (random) để message fragment thành nhiều frame thật.
	big := make([]byte, 48<<10)
	if _, err := io.ReadFull(crand.Reader, big); err != nil {
		t.Fatalf("rand: %v", err)
	}

	msgs := [][]byte{
		[]byte(`{"event":"message","data":"stream tron ven mot frame"}`),
		big,
		[]byte(`{"event":"message","data":"sau message lon van phai doc tiep duoc"}`),
		[]byte(`{"event":"raw","data":"xen ke khong nen"}`),
		[]byte(`{"event":"message","data":"duoi 00 00 ff ff khong strip"}`),
		[]byte(`{"event":"message","data":"ket thuc"}`),
	}

	f := startFakeZaloServer(t, func(t *testing.T, conn net.Conn, br *bufio.Reader) {
		// m0: stream trọn vẹn, 1 frame FIN.
		if err := writeServerFrame(conn, true, true, opcodeText, fullDeflateStream(t, msgs[0])); err != nil {
			t.Errorf("ghi m0: %v", err)
			return
		}
		// m1: stream trọn vẹn fragment nhiều frame + FIN frame RỖNG — kịch bản
		// gây kẹt fin=false trên production.
		if err := writeFragmentedServerMessage(conn, fullDeflateStream(t, msgs[1]), 4096, true); err != nil {
			t.Errorf("ghi m1: %v", err)
			return
		}
		// m2: padding 0x00 sau block BFINAL, 1 frame.
		if err := writeServerFrame(conn, true, true, opcodeText, append(fullDeflateStream(t, msgs[2]), 0x00)); err != nil {
			t.Errorf("ghi m2: %v", err)
			return
		}
		// m3: raw xen kẽ.
		if err := writeServerFrame(conn, true, false, opcodeText, msgs[3]); err != nil {
			t.Errorf("ghi m3: %v", err)
			return
		}
		// m4: đuôi sync-flush không strip nằm lại trong frame.
		if err := writeServerFrame(conn, true, true, opcodeText, append(fullDeflateStream(t, msgs[4]), 0x00, 0x00, 0xff, 0xff)); err != nil {
			t.Errorf("ghi m4: %v", err)
			return
		}
		// m5: message thường khép lại — chứng minh stream vẫn đồng bộ.
		if err := writeServerFrame(conn, true, true, opcodeText, fullDeflateStream(t, msgs[5])); err != nil {
			t.Errorf("ghi m5: %v", err)
			return
		}
		_, _ = io.Copy(io.Discard, br)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	c, _, err := websocket.Dial(ctx, f.wsURL(), &websocket.DialOptions{
		CompressionMode: websocket.CompressionContextTakeover,
		ForceDeflate:    true,
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.CloseNow()
	c.SetReadLimit(1 << 20) // payload giải nén 48KB > trần mặc định 32KB

	for i, want := range msgs {
		typ, data, err := c.Read(ctx)
		if err != nil {
			t.Fatalf("doc msg %d: %v", i, err)
		}
		if typ != websocket.MessageText {
			t.Fatalf("msg %d type = %v, mong MessageText", i, typ)
		}
		if !bytes.Equal(data, want) {
			t.Fatalf("msg %d lech noi dung (len=%d, mong %d)", i, len(data), len(want))
		}
	}
}

// TestForceDeflateContextTakeoverFragmentedStream — biến thể RFC 7692 CHUẨN
// chạy dài hơi: server giữ context takeover (MỘT flate.Writer xuyên suốt),
// sync-flush + strip đuôi 00 00 ff ff đúng §7.2.1, message lớn fragment nhiều
// frame (kèm FIN frame rỗng), xen kẽ raw — client ForceDeflate phải Read()
// lặp nhiều message liên tiếp không lỗi và đúng nội dung từng message.
func TestForceDeflateContextTakeoverFragmentedStream(t *testing.T) {
	t.Parallel()

	var msgs [][]byte
	for i := 0; i < 24; i++ {
		switch {
		case i%5 == 2:
			msgs = append(msgs, []byte(fmt.Sprintf(`{"event":"raw","seq":%d}`, i)))
		case i%4 == 3:
			msgs = append(msgs, bytes.Repeat([]byte(fmt.Sprintf(`{"event":"group_event","seq":%d,"members":"abcdefgh"}`, i)), 200))
		default:
			msgs = append(msgs, []byte(fmt.Sprintf(`{"event":"message","seq":%d,"data":"noi dung tin nhan so %d"}`, i, i)))
		}
	}

	f := startFakeZaloServer(t, func(t *testing.T, conn net.Conn, br *bufio.Reader) {
		comp := newPMCECompressor(t)
		for i, m := range msgs {
			if i%5 == 2 { // raw xen kẽ
				if err := writeServerFrame(conn, true, false, opcodeText, m); err != nil {
					t.Errorf("ghi raw %d: %v", i, err)
					return
				}
				continue
			}
			cp := comp.compress(t, m)
			if i%4 == 3 { // message lớn: fragment + FIN frame rỗng
				if err := writeFragmentedServerMessage(conn, cp, 64, true); err != nil {
					t.Errorf("ghi fragment %d: %v", i, err)
					return
				}
				continue
			}
			if err := writeServerFrame(conn, true, true, opcodeText, cp); err != nil {
				t.Errorf("ghi nen %d: %v", i, err)
				return
			}
		}
		_, _ = io.Copy(io.Discard, br)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	c, _, err := websocket.Dial(ctx, f.wsURL(), &websocket.DialOptions{
		CompressionMode: websocket.CompressionContextTakeover,
		ForceDeflate:    true,
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.CloseNow()
	c.SetReadLimit(1 << 20)

	for i, want := range msgs {
		_, data, err := c.Read(ctx)
		if err != nil {
			t.Fatalf("doc msg %d: %v", i, err)
		}
		if !bytes.Equal(data, want) {
			t.Fatalf("msg %d lech noi dung (len=%d, mong %d)", i, len(data), len(want))
		}
	}
}

// TestForceDeflateNoopWhenServerEchoesExtension — server tử tế echo extension
// thì ForceDeflate phải vô hiệu (đi đường negotiate chuẩn, chiều gửi ĐƯỢC nén).
func TestForceDeflateNoopWhenServerEchoesExtension(t *testing.T) {
	t.Parallel()

	// Server thật của bản vendor (Accept) có negotiate extension đầy đủ.
	echoSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			CompressionMode: websocket.CompressionContextTakeover,
		})
		if err != nil {
			t.Errorf("accept: %v", err)
			return
		}
		defer c.CloseNow()

		typ, data, err := c.Read(r.Context())
		if err != nil {
			return
		}
		_ = c.Write(r.Context(), typ, data) // echo
		c.CloseNow()
	}))
	t.Cleanup(echoSrv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	url := "ws" + strings.TrimPrefix(echoSrv.URL, "http")
	c, resp, err := websocket.Dial(ctx, url, &websocket.DialOptions{
		CompressionMode: websocket.CompressionContextTakeover,
		ForceDeflate:    true,
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.CloseNow()

	if got := resp.Header.Get("Sec-WebSocket-Extensions"); !strings.Contains(got, "permessage-deflate") {
		t.Fatalf("server vendor phai echo permessage-deflate, got %q", got)
	}

	want := bytes.Repeat([]byte("echo-nen-duoc-negotiate-"), 32)
	if err := c.Write(ctx, websocket.MessageText, want); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("read echo: %v", err)
	}
	if !bytes.Equal(data, want) {
		t.Fatalf("echo lech: %q != %q", data, want)
	}
}

// TestForceDeflateCorruptMessageDropsOnlyThatMessage — tái hiện lỗi production
// VÒNG 3 (PATCH FORK số 5): "failed to read: flate: corrupt input before
// offset 8" rồi burst desync ("opPong: received fragmented control frame",
// rsv đủ tổ hợp) cùng 1ms, close 1011, reconnect lặp ~47 lần/3 phút vì server
// gửi lại đúng batch độc mỗi lần connect.
//
// Payload độc dùng byte đầu 0x06 (BTYPE=11 reserved) để flate báo
// CorruptInputError NGAY và DETERMINISTIC. Yêu cầu sau vá: lỗi chỉ DROP đúng
// message độc — connection giữ đồng bộ framing, message lành kế tiếp (cả sau
// poison 1 frame lẫn poison fragmented + FIN rỗng) phải đọc được bình thường,
// KHÔNG rơi vào ErrPreviousMessageNotRead / ErrProtocolDesync.
func TestForceDeflateCorruptMessageDropsOnlyThatMessage(t *testing.T) {
	t.Parallel()

	good0 := []byte(`{"event":"message","data":"truoc poison"}`)
	good1 := []byte(`{"event":"message","data":"sau poison 1 frame van doc tiep duoc"}`)
	good2 := []byte(`{"event":"message","data":"sau poison fragmented van doc tiep duoc"}`)

	poison := []byte{0x06, 0xde, 0xad, 0xbe, 0xef} // BTYPE=11 reserved ⇒ corrupt ngay
	poisonBig := append([]byte{0x06}, bytes.Repeat([]byte{0xab}, 300)...)

	f := startFakeZaloServer(t, func(t *testing.T, conn net.Conn, br *bufio.Reader) {
		if err := writeServerFrame(conn, true, true, opcodeText, fullDeflateStream(t, good0)); err != nil {
			t.Errorf("ghi good0: %v", err)
			return
		}
		// Poison 1 frame FIN — drain phải nuốt phần payload nén còn lại.
		if err := writeServerFrame(conn, true, true, opcodeText, poison); err != nil {
			t.Errorf("ghi poison: %v", err)
			return
		}
		if err := writeServerFrame(conn, true, true, opcodeText, fullDeflateStream(t, good1)); err != nil {
			t.Errorf("ghi good1: %v", err)
			return
		}
		// Poison fragmented nhiều frame + FIN frame RỖNG — drain phải đi qua
		// đủ continuation/FIN để mr.fin không kẹt false.
		if err := writeFragmentedServerMessage(conn, poisonBig, 64, true); err != nil {
			t.Errorf("ghi poison fragmented: %v", err)
			return
		}
		if err := writeServerFrame(conn, true, true, opcodeText, fullDeflateStream(t, good2)); err != nil {
			t.Errorf("ghi good2: %v", err)
			return
		}
		_, _ = io.Copy(io.Discard, br)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	c, _, err := websocket.Dial(ctx, f.wsURL(), &websocket.DialOptions{
		CompressionMode: websocket.CompressionContextTakeover,
		ForceDeflate:    true,
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.CloseNow()

	assertCorruptDropped := func(step string) {
		t.Helper()
		_, _, err := c.Read(ctx)
		if err == nil {
			t.Fatalf("%s: mong loi corrupt input, nhung doc thanh cong", step)
		}
		var cie flate.CorruptInputError
		if !errors.As(err, &cie) {
			t.Fatalf("%s: mong flate.CorruptInputError trong chuoi loi, got: %v", step, err)
		}
		if errors.Is(err, websocket.ErrProtocolDesync) {
			t.Fatalf("%s: loi flate thuan KHONG duoc mang ErrProtocolDesync (phai non-fatal): %v", step, err)
		}
		if errors.Is(err, websocket.ErrPreviousMessageNotRead) {
			t.Fatalf("%s: ket msgReader nhu truoc khi va: %v", step, err)
		}
	}
	assertGood := func(step string, want []byte) {
		t.Helper()
		_, data, err := c.Read(ctx)
		if err != nil {
			t.Fatalf("%s: %v", step, err)
		}
		if !bytes.Equal(data, want) {
			t.Fatalf("%s: lech noi dung %q != %q", step, data, want)
		}
	}

	assertGood("good0", good0)
	assertCorruptDropped("poison 1 frame")
	assertGood("good1 sau poison", good1)
	assertCorruptDropped("poison fragmented")
	assertGood("good2 sau poison fragmented", good2)
}

// TestForceDeflateStreamSpillAcrossMessageBoundary — mô hình nghi phạm thực ở
// production vòng 3: dưới burst, edge Zalo cắt ranh giới WS message KHÔNG
// trùng ranh giới deflate stream — MỘT one-shot stream bị bửa đôi GIỮA block
// thành 2 message RSV1. Message sau bắt đầu giữa block ⇒ chính là
// "corrupt input before offset 8". Hai message độc đó không client nào decode
// nổi (drop chấp nhận được) — yêu cầu sau vá là CONNECTION SỐNG: message
// thẳng hàng phía sau phải đến nơi, không teardown, không kẹt.
func TestForceDeflateStreamSpillAcrossMessageBoundary(t *testing.T) {
	t.Parallel()

	big := make([]byte, 8<<10)
	if _, err := io.ReadFull(crand.Reader, big); err != nil {
		t.Fatalf("rand: %v", err)
	}
	spill := fullDeflateStream(t, big)
	half := len(spill) / 2

	good0 := []byte(`{"event":"message","data":"truoc burst"}`)
	good1 := []byte(`{"event":"message","data":"sau burst lech ranh gioi van phai song"}`)

	f := startFakeZaloServer(t, func(t *testing.T, conn net.Conn, br *bufio.Reader) {
		if err := writeServerFrame(conn, true, true, opcodeText, fullDeflateStream(t, good0)); err != nil {
			t.Errorf("ghi good0: %v", err)
			return
		}
		// Stream bị cắt đôi giữa block thành 2 WS message "hoàn chỉnh".
		if err := writeServerFrame(conn, true, true, opcodeText, spill[:half]); err != nil {
			t.Errorf("ghi spill dau: %v", err)
			return
		}
		if err := writeServerFrame(conn, true, true, opcodeText, spill[half:]); err != nil {
			t.Errorf("ghi spill cuoi: %v", err)
			return
		}
		if err := writeServerFrame(conn, true, true, opcodeText, fullDeflateStream(t, good1)); err != nil {
			t.Errorf("ghi good1: %v", err)
			return
		}
		_, _ = io.Copy(io.Discard, br)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	c, _, err := websocket.Dial(ctx, f.wsURL(), &websocket.DialOptions{
		CompressionMode: websocket.CompressionContextTakeover,
		ForceDeflate:    true,
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.CloseNow()
	c.SetReadLimit(1 << 20)

	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("good0: %v", err)
	}
	if !bytes.Equal(data, good0) {
		t.Fatalf("good0 lech: %q", data)
	}

	// 2 message spill: tuỳ vị trí cắt, từng message có thể trả lỗi corrupt
	// (drop) hoặc data rác (inflate trúng bit hợp lệ) — cả hai chấp nhận được,
	// CẤM duy nhất: lỗi kẹt connection. good1 phải đến trong tối đa 3 lần đọc.
	for i := 0; i < 3; i++ {
		_, data, err := c.Read(ctx)
		if err != nil {
			if errors.Is(err, websocket.ErrPreviousMessageNotRead) || errors.Is(err, websocket.ErrProtocolDesync) {
				t.Fatalf("doc %d: connection ket/desync sau spill: %v", i, err)
			}
			continue
		}
		if bytes.Equal(data, good1) {
			return // connection sống qua burst lệch ranh giới — đạt yêu cầu
		}
	}
	t.Fatal("good1 khong den sau cac message spill — connection khong hoi phuc")
}

// TestFragmentedControlFrameIsProtocolDesync — PATCH FORK số 6: control frame
// fragmented (fin=0) là signature đọc-payload-thành-header trong production
// log vòng 3 ("opPong: received fragmented control frame"). Lỗi phải mang
// sentinel ErrProtocolDesync để tầng websocketx teardown ngay, không retry.
func TestFragmentedControlFrameIsProtocolDesync(t *testing.T) {
	t.Parallel()

	f := startFakeZaloServer(t, func(t *testing.T, conn net.Conn, br *bufio.Reader) {
		if err := writeServerFrame(conn, false /*fin*/, false, opcodePong, []byte("p")); err != nil {
			return
		}
		_, _ = io.Copy(io.Discard, br)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	c, _, err := websocket.Dial(ctx, f.wsURL(), &websocket.DialOptions{
		CompressionMode: websocket.CompressionContextTakeover,
		ForceDeflate:    true,
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.CloseNow()

	_, _, err = c.Read(ctx)
	if err == nil {
		t.Fatal("mong loi fragmented control frame, nhung doc thanh cong")
	}
	if !strings.Contains(err.Error(), "received fragmented control frame") {
		t.Fatalf("mong text loi upstream giu nguyen, got: %v", err)
	}
	if !errors.Is(err, websocket.ErrProtocolDesync) {
		t.Fatalf("loi desync phai mang sentinel ErrProtocolDesync: %v", err)
	}
}
