package websocketx

import (
	"context"
	"sync"

	websocket "github.com/luannt2002/zalo-api-unofficial/internal/coderws"
)

const (
	TextMessage   = websocket.MessageText
	BinaryMessage = websocket.MessageBinary
)

type Client interface {
	Messages() <-chan Message
	Errors() <-chan error
	Closed() <-chan CloseInfo

	Write(ctx context.Context, typ websocket.MessageType, data []byte) error
	WriteText(ctx context.Context, s string) error

	Close(code int, reason string)
}

type Message struct {
	Type websocket.MessageType
	Data []byte
}

type CloseInfo struct {
	Code   int
	Reason string
	Err    error
}

type writeRequest struct {
	ctx    context.Context
	typ    websocket.MessageType
	data   []byte
	result chan error
}

type client struct {
	conn    *websocket.Conn
	connCtx context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	once    sync.Once

	msgChan    chan Message
	errChan    chan error
	closedChan chan CloseInfo
	writeQueue chan writeRequest

	logf func(string, ...any) // optional debug logger (handshake + readLoop fatal)
}

var _ Client = (*client)(nil)

func Dial(ctx context.Context, url string, opt *Options) (*client, error) {
	cfg := defaultOptions()
	if opt != nil {
		if opt.Header != nil {
			cfg.Header = opt.Header
		}
		if opt.MsgBuf > 0 {
			cfg.MsgBuf = opt.MsgBuf
		}
		if opt.ErrBuf > 0 {
			cfg.ErrBuf = opt.ErrBuf
		}
		if opt.WriteBuf > 0 {
			cfg.WriteBuf = opt.WriteBuf
		}
		cfg.ForceDeflate = opt.ForceDeflate
		cfg.Logf = opt.Logf
	}

	dialOpts := &websocket.DialOptions{
		HTTPHeader: cfg.Header,
		// BẮT BUỘC offer permessage-deflate: server WS Zalo nén frame (RSV1) theo
		// mặc cảnh client là browser (browser LUÔN offer extension này). Không offer
		// ⇒ server vẫn nén ⇒ reader lỗi "unexpected rsv bits" rồi desync thành
		// "unknown opcode" — listener chết vòng lặp, bot không nhận event nào.
		// ContextTakeover = giữ cửa sổ nén giữa các frame (khớp hành vi browser).
		CompressionMode: websocket.CompressionContextTakeover,
		// Root cause đã chốt từ production log: dù offer đúng, server Zalo vẫn
		// KHÔNG echo extension (ext="") mà cứ gửi frame nén RSV1 ⇒ phải ép bản
		// vendor giải nén chiều đọc. Xem internal/coderws/dial.go (PATCH FORK).
		ForceDeflate: cfg.ForceDeflate,
	}

	if cfg.HTTPClient != nil {
		dialOpts.HTTPClient = cfg.HTTPClient
	}

	conn, resp, err := websocket.Dial(ctx, url, dialOpts)
	if err != nil {
		return nil, err
	}
	// Log vận hành 1 dòng/lần connect: ext="" + forceDeflate=true là dấu hiệu
	// đang chạy đường vá RSV1 (server Zalo nén nhưng không echo extension).
	if resp != nil && cfg.Logf != nil {
		cfg.Logf("[websocketx] handshake status=%s ext=%q server=%q forceDeflate=%v",
			resp.Status, resp.Header.Get("Sec-WebSocket-Extensions"), resp.Header.Get("Server"), cfg.ForceDeflate)
	}
	// Nâng trần frame đọc: coder/websocket mặc định 32KB — frame Zalo lớn hơn (event nhóm
	// đông thành viên, danh sách dài) sẽ gây StatusMessageTooBig → listener chết + vòng
	// reconnect vô ích. 10MB đủ rộng cho mọi payload control/event thực tế của Zalo.
	conn.SetReadLimit(10 << 20)

	cctx, cancel := context.WithCancel(ctx)
	cl := &client{
		conn:       conn,
		connCtx:    cctx,
		cancel:     cancel,
		msgChan:    make(chan Message, cfg.MsgBuf),
		errChan:    make(chan error, cfg.ErrBuf),
		closedChan: make(chan CloseInfo, 1),
		writeQueue: make(chan writeRequest, cfg.WriteBuf),
		logf:       cfg.Logf,
	}

	// Reader
	cl.wg.Add(1)
	go cl.readLoop(cctx)

	// Writer
	cl.wg.Add(1)
	go cl.writeLoop(cctx)

	return cl, nil
}

func (c *client) Messages() <-chan Message { return c.msgChan }
func (c *client) Errors() <-chan error     { return c.errChan }
func (c *client) Closed() <-chan CloseInfo { return c.closedChan }

func (c *client) Write(ctx context.Context, typ websocket.MessageType, data []byte) error {
	req := writeRequest{
		ctx:    ctx,
		typ:    typ,
		data:   data,
		result: make(chan error, 1),
	}

	select {
	case c.writeQueue <- req:
		select {
		case err := <-req.result:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *client) WriteText(ctx context.Context, s string) error {
	return c.Write(ctx, websocket.MessageText, []byte(s))
}

func (c *client) Close(code int, reason string) {
	c.shutdown(CloseInfo{Code: code, Reason: reason}, true)
}

func (c *client) readLoop(ctx context.Context) {
	defer c.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		typ, data, err := c.conn.Read(ctx)
		if err != nil {
			if c.isFatalErr(err) {
				// DEBUG: surface the REAL underlying read error (otherwise upper
				// layers only see code 1011 + empty reason from closeInfoFromErr).
				if c.logf != nil {
					c.logf("[websocketx] readLoop FATAL: %v", err)
				}
				c.shutdown(closeInfoFromErr(err), false)
				return
			} else {
				c.handleErr(err)
				continue
			}
		}

		switch typ {
		case websocket.MessageText, websocket.MessageBinary:
			c.handleMsg(Message{Type: typ, Data: data})
		}
	}
}

func (c *client) writeLoop(ctx context.Context) {
	defer c.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case req := <-c.writeQueue:
			err := c.conn.Write(req.ctx, req.typ, req.data)

			select {
			case req.result <- err:
			default:
			}

			if err != nil && c.isFatalErr(err) {
				c.shutdown(closeInfoFromErr(err), false)
				return
			}
		}
	}
}

func (c *client) shutdown(ci CloseInfo, sendCloseFrame bool) {
	c.once.Do(func() {
		c.pushClose(ci)
		if sendCloseFrame {
			// Close handshake được chặn trần bởi timeout nội bộ của bản vendor
			// (writeClose 5s + waitCloseHandshake 5s — gồm cả PATCH FORK
			// drainPayload trong internal/coderws/close.go, nếu không stream
			// desync sẽ làm Close treo vô hạn). Bản cũ tạo ctx 2s ở đây nhưng
			// vứt đi không dùng — conn.Close không nhận ctx.
			_ = c.conn.Close(websocket.StatusCode(ci.Code), ci.Reason)
		} else {
			_ = c.conn.CloseNow()
		}
		c.cancel()
		go func() {
			c.wg.Wait()
			close(c.msgChan)
			close(c.errChan)
			close(c.closedChan)
		}()
	})
}

func (c *client) handleMsg(m Message) {
	select {
	case c.msgChan <- m:
	default:
		select { // drop oldest
		case <-c.msgChan:
		default:
		}
		select { // retry once, non-blocking
		case c.msgChan <- m:
		default:
		}
	}
}

func (c *client) handleErr(err error) {
	select {
	case c.errChan <- err:
	default:
		select { // drop oldest
		case <-c.errChan:
		default:
		}
		select { // retry once, non-blocking
		case c.errChan <- err:
		default:
		}
	}
}

func (c *client) pushClose(ci CloseInfo) {
	for {
		select {
		case c.closedChan <- ci:
			return
		default:
			select {
			case <-c.closedChan:
			default:
			}
		}
	}
}
