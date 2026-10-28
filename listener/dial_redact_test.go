package listener

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strings"
	"testing"
)

// URL ws mang tham số phiên (zpw_sek...) trong query. Lỗi quay số không được in query đó ra
// Error() — ứng dụng log lỗi là lộ phiên. Giữ scheme+host+path để vẫn chẩn đoán được.
func TestDialLoiKhongLoQuery(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close() // cổng đóng ⇒ connection refused

	ln := newLiveListener(t, "ws://"+addr+"/ws/path?zpw_sek=bimat", nil)
	err = ln.Start(context.Background(), false)
	if err == nil {
		t.Fatal("Start vào cổng đóng phải lỗi")
	}
	msg := err.Error()
	if strings.Contains(msg, "bimat") {
		t.Fatalf("Error() lộ query phiên: %s", msg)
	}
	if !strings.Contains(msg, addr+"/ws/path") {
		t.Fatalf("Error() phải giữ host+path để chẩn đoán: %s", msg)
	}
	var ue *url.Error
	if errors.As(err, &ue) && strings.Contains(ue.URL, "bimat") {
		t.Fatalf("*url.Error.URL vẫn giữ query: %s", ue.URL)
	}
}
