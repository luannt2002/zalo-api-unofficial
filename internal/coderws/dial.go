//go:build !js
// +build !js

package websocket

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/luannt2002/zalo-api-unofficial/internal/coderws/internal/errd"
)

// DialOptions represents Dial's options.
type DialOptions struct {
	// HTTPClient is used for the connection.
	// Its Transport must return writable bodies for WebSocket handshakes.
	// http.Transport does beginning with Go 1.12.
	HTTPClient *http.Client

	// HTTPHeader specifies the HTTP headers included in the handshake request.
	HTTPHeader http.Header

	// Host optionally overrides the Host HTTP header to send. If empty, the value
	// of URL.Host will be used.
	Host string

	// Subprotocols lists the WebSocket subprotocols to negotiate with the server.
	Subprotocols []string

	// CompressionMode controls the compression mode.
	// Defaults to CompressionDisabled.
	//
	// See docs on CompressionMode for details.
	CompressionMode CompressionMode

	// CompressionThreshold controls the minimum size of a message before compression is applied.
	//
	// Defaults to 512 bytes for CompressionNoContextTakeover and 128 bytes
	// for CompressionContextTakeover.
	CompressionThreshold int

	// ForceDeflate — PATCH FORK (khác biệt wire-protocol QUAN TRỌNG NHẤT so với
	// upstream coder/websocket; khi re-baseline TUYỆT ĐỐI không được revert).
	//
	// Root cause (đã xác nhận bằng production log): server WebSocket của Zalo
	// nén permessage-deflate VÔ ĐIỀU KIỆN (nó mặc định client là browser — mà
	// browser luôn offer extension này) nhưng KHÔNG echo
	// "Sec-WebSocket-Extensions" trong response 101 (ext="" dù handshake OK).
	// Theo RFC 7692, không echo nghĩa là KHÔNG negotiate ⇒ mọi lib tuân chuẩn
	// từ chối frame RSV1 ("received header with unexpected rsv bits"), stream
	// đọc lệch ⇒ cascade rsv/opcode rác ⇒ close 1011 ⇒ reconnect lặp vô hạn.
	//
	// Bật flag này: nếu server KHÔNG echo extension thì client vẫn khởi tạo
	// flate phía ĐỌC ở chế độ context-takeover (như thể đã negotiate — đúng
	// giả định của server Zalo với browser). Semantics per-message GIỮ CHUẨN
	// permessage-deflate: chỉ giải nén message có RSV1 trên frame đầu, message
	// không RSV1 đọc raw. Chiều GỬI: KHÔNG nén (an toàn — server không khai gì
	// nên không có cơ sở nó nhận được dữ liệu nén từ client).
	//
	// Nếu server echo extension đàng hoàng thì flag này vô hiệu (đi đường
	// negotiate chuẩn như upstream).
	//
	// GIỮ context-takeover cho chiều đọc, KHÔNG chuyển no-context-takeover:
	// resetFlate() vốn Reset inflater sạch cho từng message (one-shot stream
	// BFINAL của Zalo decode đúng), còn dictionary chỉ NỚI LỎNG kiểm tra
	// distance — chiều đọc context-takeover chấp nhận SUPERSET của
	// no-context-takeover, đồng thời vẫn đọc được nếu Zalo đổi sang context
	// takeover thật. Lỗi message nén độc (corrupt input) xử lý ở patch số 5
	// (read.go/msgReader.Read), không phải bằng chế độ takeover.
	ForceDeflate bool
}

func (opts *DialOptions) cloneWithDefaults(ctx context.Context) (context.Context, context.CancelFunc, *DialOptions) {
	var cancel context.CancelFunc

	var o DialOptions
	if opts != nil {
		o = *opts
	}
	if o.HTTPClient == nil {
		o.HTTPClient = http.DefaultClient
	}
	if o.HTTPClient.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, o.HTTPClient.Timeout)

		newClient := *o.HTTPClient
		newClient.Timeout = 0
		o.HTTPClient = &newClient
	}
	if o.HTTPHeader == nil {
		o.HTTPHeader = http.Header{}
	}
	newClient := *o.HTTPClient
	oldCheckRedirect := o.HTTPClient.CheckRedirect
	newClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		switch req.URL.Scheme {
		case "ws":
			req.URL.Scheme = "http"
		case "wss":
			req.URL.Scheme = "https"
		}
		if oldCheckRedirect != nil {
			return oldCheckRedirect(req, via)
		}
		return nil
	}
	o.HTTPClient = &newClient

	return ctx, cancel, &o
}

// Dial performs a WebSocket handshake on url.
//
// The response is the WebSocket handshake response from the server.
// You never need to close resp.Body yourself.
//
// If an error occurs, the returned response may be non nil.
// However, you can only read the first 1024 bytes of the body.
//
// This function requires at least Go 1.12 as it uses a new feature
// in net/http to perform WebSocket handshakes.
// See docs on the HTTPClient option and https://github.com/golang/go/issues/26937#issuecomment-415855861
//
// URLs with http/https schemes will work and are interpreted as ws/wss.
func Dial(ctx context.Context, u string, opts *DialOptions) (*Conn, *http.Response, error) {
	return dial(ctx, u, opts, nil)
}

func dial(ctx context.Context, urls string, opts *DialOptions, rand io.Reader) (_ *Conn, _ *http.Response, err error) {
	defer errd.Wrap(&err, "failed to WebSocket dial")

	var cancel context.CancelFunc
	ctx, cancel, opts = opts.cloneWithDefaults(ctx)
	if cancel != nil {
		defer cancel()
	}

	secWebSocketKey, err := secWebSocketKey(rand)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate Sec-WebSocket-Key: %w", err)
	}

	var copts *compressionOptions
	if opts.CompressionMode != CompressionDisabled {
		copts = opts.CompressionMode.opts()
	}

	resp, err := handshakeRequest(ctx, urls, opts, copts, secWebSocketKey)
	if err != nil {
		return nil, resp, err
	}
	respBody := resp.Body
	resp.Body = nil
	defer func() {
		if err != nil {
			// We read a bit of the body for easier debugging.
			r := io.LimitReader(respBody, 1024)

			timer := time.AfterFunc(time.Second*3, func() {
				respBody.Close()
			})
			defer timer.Stop()

			b, _ := io.ReadAll(r)
			respBody.Close()
			resp.Body = io.NopCloser(bytes.NewReader(b))
		}
	}()

	copts, err = verifyServerResponse(opts, copts, secWebSocketKey, resp)
	if err != nil {
		return nil, resp, err
	}

	// PATCH FORK (Zalo) — điểm vá số 2 (xem doc của DialOptions.ForceDeflate):
	// server trả 101 với ext="" nhưng vẫn gửi frame nén RSV1. Ép bật flate phía
	// ĐỌC như thể đã negotiate context-takeover 2 chiều (zero-value của
	// compressionOptions = giữ cửa sổ nén giữa các message, khớp hành vi server
	// Zalo giả định với browser). Phía GHI bị khoá nén qua flateWriteDisabled.
	flateWriteDisabled := false
	if copts == nil && opts.ForceDeflate {
		copts = &compressionOptions{}
		flateWriteDisabled = true
	}

	rwc, ok := respBody.(io.ReadWriteCloser)
	if !ok {
		return nil, resp, fmt.Errorf("response body is not a io.ReadWriteCloser: %T", respBody)
	}

	return newConn(connConfig{
		subprotocol:        resp.Header.Get("Sec-WebSocket-Protocol"),
		rwc:                rwc,
		client:             true,
		copts:              copts,
		flateThreshold:     opts.CompressionThreshold,
		flateWriteDisabled: flateWriteDisabled,
		br:                 getBufioReader(rwc),
		bw:                 getBufioWriter(rwc),
	}), resp, nil
}

func handshakeRequest(ctx context.Context, urls string, opts *DialOptions, copts *compressionOptions, secWebSocketKey string) (*http.Response, error) {
	u, err := url.Parse(urls)
	if err != nil {
		return nil, fmt.Errorf("failed to parse url: %w", redactURLError(err))
	}

	switch u.Scheme {
	case "ws":
		u.Scheme = "http"
	case "wss":
		u.Scheme = "https"
	case "http", "https":
	default:
		return nil, fmt.Errorf("unexpected url scheme: %q", u.Scheme)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create new http request: %w", err)
	}
	if len(opts.Host) > 0 {
		req.Host = opts.Host
	}
	req.Header = opts.HTTPHeader.Clone()
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", secWebSocketKey)
	if len(opts.Subprotocols) > 0 {
		req.Header.Set("Sec-WebSocket-Protocol", strings.Join(opts.Subprotocols, ","))
	}
	if copts != nil {
		// PATCH FORK (Zalo) — điểm vá số 1: offer extension GIỐNG HỆT browser
		// (Chrome): "permessage-deflate; client_max_window_bits". Upstream chỉ
		// gửi "permessage-deflate" — edge Zalo phân nhánh hành vi theo offer
		// của browser nên phải khớp nguyên văn format này.
		req.Header.Set("Sec-WebSocket-Extensions", copts.String()+"; client_max_window_bits")
	}

	resp, err := opts.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send handshake request: %w", redactURLError(err))
	}
	return resp, nil
}

// PATCH FORK (Zalo): *url.Error prints the whole URL, and the Zalo ws query
// carries session parameters (zpw_sek...). Keep scheme, host and path only.
func redactURLError(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		if i := strings.IndexAny(ue.URL, "?#"); i >= 0 {
			ue.URL = ue.URL[:i] + "?<redacted>"
		}
	}
	return err
}

func secWebSocketKey(rr io.Reader) (string, error) {
	if rr == nil {
		rr = rand.Reader
	}
	b := make([]byte, 16)
	_, err := io.ReadFull(rr, b)
	if err != nil {
		return "", fmt.Errorf("failed to read random data from rand.Reader: %w", err)
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

func verifyServerResponse(opts *DialOptions, copts *compressionOptions, secWebSocketKey string, resp *http.Response) (*compressionOptions, error) {
	if resp.StatusCode != http.StatusSwitchingProtocols {
		return nil, fmt.Errorf("expected handshake response status code %v but got %v", http.StatusSwitchingProtocols, resp.StatusCode)
	}

	if !headerContainsTokenIgnoreCase(resp.Header, "Connection", "Upgrade") {
		return nil, fmt.Errorf("WebSocket protocol violation: Connection header %q does not contain Upgrade", resp.Header.Get("Connection"))
	}

	if !headerContainsTokenIgnoreCase(resp.Header, "Upgrade", "WebSocket") {
		return nil, fmt.Errorf("WebSocket protocol violation: Upgrade header %q does not contain websocket", resp.Header.Get("Upgrade"))
	}

	if resp.Header.Get("Sec-WebSocket-Accept") != secWebSocketAccept(secWebSocketKey) {
		return nil, fmt.Errorf("WebSocket protocol violation: invalid Sec-WebSocket-Accept %q, key %q",
			resp.Header.Get("Sec-WebSocket-Accept"),
			secWebSocketKey,
		)
	}

	err := verifySubprotocol(opts.Subprotocols, resp)
	if err != nil {
		return nil, err
	}

	return verifyServerExtensions(copts, resp.Header)
}

func verifySubprotocol(subprotos []string, resp *http.Response) error {
	proto := resp.Header.Get("Sec-WebSocket-Protocol")
	if proto == "" {
		return nil
	}

	for _, sp2 := range subprotos {
		if strings.EqualFold(sp2, proto) {
			return nil
		}
	}

	return fmt.Errorf("WebSocket protocol violation: unexpected Sec-WebSocket-Protocol from server: %q", proto)
}

func verifyServerExtensions(copts *compressionOptions, h http.Header) (*compressionOptions, error) {
	exts := websocketExtensions(h)
	if len(exts) == 0 {
		return nil, nil
	}

	ext := exts[0]
	if ext.name != "permessage-deflate" || len(exts) > 1 || copts == nil {
		return nil, fmt.Errorf("WebSocket protcol violation: unsupported extensions from server: %+v", exts[1:])
	}

	_copts := *copts
	copts = &_copts

	for _, p := range ext.params {
		switch p {
		case "client_no_context_takeover":
			copts.clientNoContextTakeover = true
			continue
		case "server_no_context_takeover":
			copts.serverNoContextTakeover = true
			continue
		}
		if strings.HasPrefix(p, "server_max_window_bits=") {
			// We can't adjust the deflate window, but decoding with a larger window is acceptable.
			continue
		}
		// PATCH FORK (Zalo): vì client offer "client_max_window_bits" (giống
		// browser) nên server chuẩn được phép echo lại tham số này. Chỉ chấp
		// nhận bare hoặc =15 (compress/flate luôn nén cửa sổ 15 bit, không
		// thu nhỏ được); giá trị khác rơi xuống lỗi bên dưới.
		if p == "client_max_window_bits" || p == "client_max_window_bits=15" {
			continue
		}

		return nil, fmt.Errorf("unsupported permessage-deflate parameter: %q", p)
	}

	return copts, nil
}

var bufioReaderPool sync.Pool

func getBufioReader(r io.Reader) *bufio.Reader {
	br, ok := bufioReaderPool.Get().(*bufio.Reader)
	if !ok {
		return bufio.NewReader(r)
	}
	br.Reset(r)
	return br
}

func putBufioReader(br *bufio.Reader) {
	bufioReaderPool.Put(br)
}

var bufioWriterPool sync.Pool

func getBufioWriter(w io.Writer) *bufio.Writer {
	bw, ok := bufioWriterPool.Get().(*bufio.Writer)
	if !ok {
		return bufio.NewWriter(w)
	}
	bw.Reset(w)
	return bw
}

func putBufioWriter(bw *bufio.Writer) {
	bufioWriterPool.Put(bw)
}
