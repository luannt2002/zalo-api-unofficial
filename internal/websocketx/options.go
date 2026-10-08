package websocketx

import (
	"net/http"
)

type Options struct {
	Header     http.Header
	HTTPClient *http.Client
	MsgBuf     int
	ErrBuf     int
	WriteBuf   int

	// ForceDeflate: server WS Zalo nén permessage-deflate vô điều kiện nhưng
	// không echo extension trong handshake — bật flag này để bản vendor
	// (internal/coderws) vẫn giải nén frame RSV1 dù handshake trả ext="".
	// Xem doc DialOptions.ForceDeflate trong internal/coderws/dial.go.
	ForceDeflate bool

	// Logf: log 1 dòng kết quả handshake mỗi lần connect (status + extension
	// server echo) — hữu ích vận hành khi chẩn đoán lại sự cố nén/RSV1.
	// Nil ⇒ không log.
	Logf func(format string, args ...any)
}

func defaultOptions() Options {
	return Options{
		MsgBuf:   64,
		ErrBuf:   8,
		WriteBuf: 16,
	}
}
