//go:build !js
// +build !js

package websocket

import (
	"bufio"
	"compress/flate"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/luannt2002/zalo-api-unofficial/internal/coderws/internal/errd"
	"github.com/luannt2002/zalo-api-unofficial/internal/coderws/internal/util"
	"github.com/luannt2002/zalo-api-unofficial/internal/coderws/internal/xsync"
)

// Reader reads from the connection until there is a WebSocket
// data message to be read. It will handle ping, pong and close frames as appropriate.
//
// It returns the type of the message and an io.Reader to read it.
// The passed context will also bound the reader.
// Ensure you read to EOF otherwise the connection will hang.
//
// Call CloseRead if you do not expect any data messages from the peer.
//
// Only one Reader may be open at a time.
//
// If you need a separate timeout on the Reader call and the Read itself,
// use time.AfterFunc to cancel the context passed in.
// See https://github.com/nhooyr/websocket/issues/87#issue-451703332
// Most users should not need this.
func (c *Conn) Reader(ctx context.Context) (MessageType, io.Reader, error) {
	return c.reader(ctx)
}

// Read is a convenience method around Reader to read a single message
// from the connection.
func (c *Conn) Read(ctx context.Context) (MessageType, []byte, error) {
	typ, r, err := c.Reader(ctx)
	if err != nil {
		return 0, nil, err
	}

	b, err := io.ReadAll(r)
	return typ, b, err
}

// CloseRead starts a goroutine to read from the connection until it is closed
// or a data message is received.
//
// Once CloseRead is called you cannot read any messages from the connection.
// The returned context will be cancelled when the connection is closed.
//
// If a data message is received, the connection will be closed with StatusPolicyViolation.
//
// Call CloseRead when you do not expect to read any more messages.
// Since it actively reads from the connection, it will ensure that ping, pong and close
// frames are responded to. This means c.Ping and c.Close will still work as expected.
//
// This function is idempotent.
func (c *Conn) CloseRead(ctx context.Context) context.Context {
	c.closeReadMu.Lock()
	ctx2 := c.closeReadCtx
	if ctx2 != nil {
		c.closeReadMu.Unlock()
		return ctx2
	}
	ctx, cancel := context.WithCancel(ctx)
	c.closeReadCtx = ctx
	c.closeReadDone = make(chan struct{})
	c.closeReadMu.Unlock()

	go func() {
		defer close(c.closeReadDone)
		defer cancel()
		defer c.close()
		_, _, err := c.Reader(ctx)
		if err == nil {
			c.Close(StatusPolicyViolation, "unexpected data message")
		}
	}()
	return ctx
}

// SetReadLimit sets the max number of bytes to read for a single message.
// It applies to the Reader and Read methods.
//
// By default, the connection has a message read limit of 32768 bytes.
//
// When the limit is hit, the connection will be closed with StatusMessageTooBig.
//
// Set to -1 to disable.
func (c *Conn) SetReadLimit(n int64) {
	if n >= 0 {
		// We read one more byte than the limit in case
		// there is a fin frame that needs to be read.
		n++
	}

	c.msgReader.limitReader.limit.Store(n)
}

const defaultReadLimit = 32768

func newMsgReader(c *Conn) *msgReader {
	mr := &msgReader{
		c:   c,
		fin: true,
	}
	mr.readFunc = mr.read

	mr.limitReader = newLimitReader(c, mr.readFunc, defaultReadLimit+1)
	return mr
}

func (mr *msgReader) resetFlate() {
	if mr.flateContextTakeover() {
		if mr.dict == nil {
			mr.dict = &slidingWindow{}
		}
		mr.dict.init(32768)
	}
	if mr.flateBufio == nil {
		mr.flateBufio = getBufioReader(mr.readFunc)
	}

	if mr.flateContextTakeover() {
		mr.flateReader = getFlateReader(mr.flateBufio, mr.dict.buf)
	} else {
		mr.flateReader = getFlateReader(mr.flateBufio, nil)
	}
	mr.limitReader.r = mr.flateReader
	mr.flateTail.Reset(deflateMessageTail)
}

func (mr *msgReader) putFlateReader() {
	if mr.flateReader != nil {
		putFlateReader(mr.flateReader)
		mr.flateReader = nil
	}
}

func (mr *msgReader) close() {
	mr.c.readMu.forceLock()
	mr.putFlateReader()
	if mr.dict != nil {
		mr.dict.close()
		mr.dict = nil
	}
	if mr.flateBufio != nil {
		putBufioReader(mr.flateBufio)
	}

	if mr.c.client {
		putBufioReader(mr.c.br)
		mr.c.br = nil
	}
}

func (mr *msgReader) flateContextTakeover() bool {
	if mr.c.client {
		return !mr.c.copts.serverNoContextTakeover
	}
	return !mr.c.copts.clientNoContextTakeover
}

func (c *Conn) readRSV1Illegal(h header) bool {
	// If compression is disabled, rsv1 is illegal.
	if !c.flate() {
		return true
	}
	// rsv1 is only allowed on data frames beginning messages.
	if h.opcode != opText && h.opcode != opBinary {
		return true
	}
	return false
}

func (c *Conn) readLoop(ctx context.Context) (header, error) {
	for {
		h, err := c.readFrameHeader(ctx)
		if err != nil {
			return header{}, err
		}

		if h.rsv1 && c.readRSV1Illegal(h) || h.rsv2 || h.rsv3 {
			// PATCH FORK (Zalo): gắn ErrProtocolDesync — header với rsv rác trên
			// đường client gần như chắc chắn là đọc nhầm payload thành header.
			err := desync(fmt.Errorf("received header with unexpected rsv bits set: %v:%v:%v", h.rsv1, h.rsv2, h.rsv3))
			c.writeError(StatusProtocolError, err)
			return header{}, err
		}

		if !c.client && !h.masked {
			return header{}, errors.New("received unmasked frame from client")
		}

		switch h.opcode {
		case opClose, opPing, opPong:
			err = c.handleControl(ctx, h)
			if err != nil {
				// Pass through CloseErrors when receiving a close frame.
				if h.opcode == opClose && CloseStatus(err) != -1 {
					return header{}, err
				}
				return header{}, fmt.Errorf("failed to handle control frame %v: %w", h.opcode, err)
			}
		case opContinuation, opText, opBinary:
			return h, nil
		default:
			// PATCH FORK (Zalo): gắn ErrProtocolDesync (xem nhánh rsv ở trên).
			err := desync(fmt.Errorf("received unknown opcode %v", h.opcode))
			c.writeError(StatusProtocolError, err)
			return header{}, err
		}
	}
}

func (c *Conn) readFrameHeader(ctx context.Context) (header, error) {
	select {
	case <-c.closed:
		return header{}, net.ErrClosed
	case c.readTimeout <- ctx:
	}

	h, err := readFrameHeader(c.br, c.readHeaderBuf[:])
	if err != nil {
		select {
		case <-c.closed:
			return header{}, net.ErrClosed
		case <-ctx.Done():
			return header{}, ctx.Err()
		default:
			return header{}, err
		}
	}

	select {
	case <-c.closed:
		return header{}, net.ErrClosed
	case c.readTimeout <- context.Background():
	}

	return h, nil
}

func (c *Conn) readFramePayload(ctx context.Context, p []byte) (int, error) {
	select {
	case <-c.closed:
		return 0, net.ErrClosed
	case c.readTimeout <- ctx:
	}

	n, err := io.ReadFull(c.br, p)
	if err != nil {
		select {
		case <-c.closed:
			return n, net.ErrClosed
		case <-ctx.Done():
			return n, ctx.Err()
		default:
			return n, fmt.Errorf("failed to read frame payload: %w", err)
		}
	}

	select {
	case <-c.closed:
		return n, net.ErrClosed
	case c.readTimeout <- context.Background():
	}

	return n, err
}

func (c *Conn) handleControl(ctx context.Context, h header) (err error) {
	// PATCH FORK (Zalo): 2 nhánh dưới gắn ErrProtocolDesync. Control frame theo
	// RFC 6455 §5.5 KHÔNG BAO GIỜ fragmented (fin=0) hay dài quá 125 byte — gặp
	// dạng này trên đường client nghĩa là đang đọc payload như header (đúng
	// signature production "opPong: received fragmented control frame": byte
	// 0x0A giữa payload bị hiểu thành fin=0|opcode=pong). Lưu ý đường control
	// frame KHÔNG đi qua flate: payload đọc thẳng readFramePayload từ c.br.
	if h.payloadLength < 0 || h.payloadLength > maxControlPayload {
		err := desync(fmt.Errorf("received control frame payload with invalid length: %d", h.payloadLength))
		c.writeError(StatusProtocolError, err)
		return err
	}

	if !h.fin {
		err := desync(errors.New("received fragmented control frame"))
		c.writeError(StatusProtocolError, err)
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	b := c.readControlBuf[:h.payloadLength]
	_, err = c.readFramePayload(ctx, b)
	if err != nil {
		return err
	}

	if h.masked {
		mask(b, h.maskKey)
	}

	switch h.opcode {
	case opPing:
		return c.writeControl(ctx, opPong, b)
	case opPong:
		c.activePingsMu.Lock()
		pong, ok := c.activePings[string(b)]
		c.activePingsMu.Unlock()
		if ok {
			select {
			case pong <- struct{}{}:
			default:
			}
		}
		return nil
	}

	// opClose

	ce, err := parseClosePayload(b)
	if err != nil {
		// PATCH FORK (Zalo): gắn ErrProtocolDesync — close payload rác cũng là
		// hệ quả đọc lệch stream; connection không tiếp tục được nữa.
		err = desync(fmt.Errorf("received invalid close payload: %w", err))
		c.writeError(StatusProtocolError, err)
		return err
	}

	err = fmt.Errorf("received close frame: %w", ce)
	c.writeClose(ce.Code, ce.Reason)
	c.readMu.unlock()
	c.close()
	return err
}

// ErrPreviousMessageNotRead — PATCH FORK (Zalo): sentinel cho lỗi "previous
// message not read to completion" (giữ nguyên text của upstream) để tầng trên
// (internal/websocketx) nhận diện bằng errors.Is và coi là FATAL: khi
// msgReader.fin kẹt false thì connection KHÔNG bao giờ tự tiến triển được nữa
// (Reader() lỗi ngay trước khi đọc byte nào) — retry chỉ tạo vòng lặp lỗi dày
// đặc nhiều dòng/ms như production log, phải teardown + reconnect.
var ErrPreviousMessageNotRead = errors.New("previous message not read to completion")

// ErrProtocolDesync — PATCH FORK (Zalo): sentinel đánh dấu lỗi "stream byte đã
// lệch khỏi ranh giới frame" (header rác: rsv bits lạ, opcode lạ, control frame
// fragmented/quá dài, continuation sai chỗ). Sau điểm này MỌI header đọc tiếp
// đều là rác — retry chỉ tạo burst lỗi dày đặc cùng-mili-giây như production
// log vòng 3 ("opPong: received fragmented control frame", rsv đủ tổ hợp).
// Tầng trên (internal/websocketx) nhận diện bằng errors.Is và coi là FATAL:
// teardown + reconnect ngay từ lỗi đầu tiên.
//
// Không đổi message gốc của upstream: desyncError giữ nguyên Error() text,
// chỉ gắn thêm nhận diện errors.Is(err, ErrProtocolDesync).
var ErrProtocolDesync = errors.New("websocket byte stream desynced from frame boundaries")

type desyncError struct{ err error }

func desync(err error) error { return &desyncError{err: err} }

func (e *desyncError) Error() string        { return e.err.Error() }
func (e *desyncError) Unwrap() error        { return e.err }
func (e *desyncError) Is(target error) bool { return target == ErrProtocolDesync }

func (c *Conn) reader(ctx context.Context) (_ MessageType, _ io.Reader, err error) {
	defer errd.Wrap(&err, "failed to get reader")

	err = c.readMu.lock(ctx)
	if err != nil {
		return 0, nil, err
	}
	defer c.readMu.unlock()

	if !c.msgReader.fin {
		return 0, nil, ErrPreviousMessageNotRead
	}

	h, err := c.readLoop(ctx)
	if err != nil {
		return 0, nil, err
	}

	if h.opcode == opContinuation {
		// PATCH FORK (Zalo): gắn ErrProtocolDesync (xem chú thích sentinel).
		err := desync(errors.New("received continuation frame without text or binary frame"))
		c.writeError(StatusProtocolError, err)
		return 0, nil, err
	}

	c.msgReader.reset(ctx, h)

	return MessageType(h.opcode), c.msgReader, nil
}

type msgReader struct {
	c *Conn

	ctx         context.Context
	flate       bool
	flateReader io.Reader
	flateBufio  *bufio.Reader
	flateTail   strings.Reader
	limitReader *limitReader
	dict        *slidingWindow

	fin           bool
	payloadLength int64
	maskKey       uint32

	// util.ReaderFunc(mr.Read) to avoid continuous allocations.
	readFunc util.ReaderFunc
}

func (mr *msgReader) reset(ctx context.Context, h header) {
	mr.ctx = ctx
	mr.flate = h.rsv1
	mr.limitReader.reset(mr.readFunc)

	if mr.flate {
		mr.resetFlate()
	}

	mr.setFrame(h)
}

func (mr *msgReader) setFrame(h header) {
	mr.fin = h.fin
	mr.payloadLength = h.payloadLength
	mr.maskKey = h.maskKey
}

func (mr *msgReader) Read(p []byte) (n int, err error) {
	err = mr.c.readMu.lock(mr.ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to read: %w", err)
	}
	defer mr.c.readMu.unlock()

	n, err = mr.limitReader.Read(p)
	if mr.flate && mr.flateContextTakeover() {
		p = p[:n]
		mr.dict.write(p)
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) && mr.fin && mr.flate {
		if mr.flate {
			// PATCH FORK (Zalo) — root cause vòng lặp production "failed to get
			// reader: previous message not read to completion": server Zalo nén
			// message thành một deflate stream TRỌN VẸN (block cuối BFINAL=1,
			// kiểu Z_FINISH) thay vì sync-flush + strip đuôi 00 00 ff ff theo
			// RFC 7692 §7.2.1. Inflater vì thế trả io.EOF NGAY khi hết block
			// cuối — TRƯỚC khi tiêu thụ phần còn lại của message WebSocket:
			// byte padding sau block cuối (§7.2.3.6), đuôi 00 00 ff ff không
			// strip, và nhất là FIN frame RỖNG của message fragmented. Header
			// FIN chưa được đọc ⇒ mr.fin kẹt false ⇒ mọi Reader() sau lỗi vĩnh
			// viễn; byte thừa nằm lại trong flateBufio còn làm hỏng message nén
			// kế tiếp. Browser (client mà server Zalo nhắm tới) inflate cả
			// buffer message và BỎ QUA phần thừa nên không sao — mô phỏng đúng
			// hành vi đó: drain nốt phần còn lại của message trước khi báo EOF.
			if derr := mr.drainFlateLeftover(maxFlateLeftoverTail); derr != nil {
				return n, fmt.Errorf("failed to read: %w", derr)
			}
		}
		mr.putFlateReader()
		return n, io.EOF
	}
	if err != nil {
		// PATCH FORK (Zalo) — root cause burst production vòng 3 ("failed to
		// read: flate: corrupt input before offset 8" + "opPong: received
		// fragmented control frame" + rsv đủ tổ hợp, lặp ~47 lần / 3 phút):
		//
		// LƯU Ý chẩn đoán: KHÔNG phải do giữ state inflater giữa các message —
		// resetFlate() đã Reset inflater sạch cho TỪNG message nén (xem
		// getFlateReader), và TestForceDeflateFullStreamMessages chứng minh các
		// one-shot stream (BFINAL) liên tiếp decode tốt. "corrupt input before
		// offset 8" nghĩa là byte ĐẦU của message không phải đầu deflate stream
		// hợp lệ: dưới burst, edge Zalo cắt ranh giới WS message KHÔNG trùng
		// ranh giới stream nén (stream tràn sang message kế ⇒ message kế bắt
		// đầu giữa block). Message độc đó không client nào decode nổi — nhưng
		// FRAMING vẫn nguyên vẹn: lỗi nằm ở tầng flate, không phải tầng frame.
		//
		// Bug cũ: abort giữa message ⇒ phần payload nén còn lại kẹt trong
		// flateBufio + trên wire ⇒ Reader() kế tiếp đọc header từ GIỮA payload
		// cũ ⇒ cascade "fragmented control frame"/rsv rác ⇒ close 1011 ⇒
		// reconnect ⇒ server gửi lại đúng batch độc ⇒ lặp vô hạn.
		//
		// Fix: lỗi flate thuần (CorruptInputError/InternalError) thì DRAIN nốt
		// phần còn lại của message ở tầng framing (cùng cơ chế với patch số 4),
		// trả lỗi cho caller để DROP đúng 1 message — connection giữ đồng bộ,
		// message kế (stream mới thẳng hàng) decode bình thường. Tầng
		// websocketx giữ phân loại non-fatal cho lỗi này (chỉ log, không
		// teardown). Nếu chính drain gặp lỗi (framing cũng hỏng) thì trả lỗi
		// drain — các site desync đã gắn ErrProtocolDesync nên tầng trên
		// teardown sạch ngay lỗi đầu tiên.
		var (
			corruptErr  flate.CorruptInputError
			internalErr flate.InternalError
		)
		if mr.flate && (errors.As(err, &corruptErr) || errors.As(err, &internalErr)) {
			if derr := mr.drainFlateLeftover(maxFlateLeftoverCorrupt); derr != nil {
				return n, fmt.Errorf("failed to read: %w", errors.Join(err, derr))
			}
			mr.putFlateReader()
			return n, fmt.Errorf("failed to read: dropped undecodable permessage-deflate message (frame stream resynced): %w", err)
		}
		return n, fmt.Errorf("failed to read: %w", err)
	}
	return n, nil
}

const (
	// maxFlateLeftoverTail — trần drain cho đường EOF bình thường: phần thừa
	// hợp lệ sau cuối stream chỉ vài byte padding/đuôi sync-flush + header FIN
	// frame rỗng.
	maxFlateLeftoverTail = 1 << 16
	// maxFlateLeftoverCorrupt — trần drain cho đường drop-message-corrupt:
	// phải nuốt được TOÀN BỘ phần nén còn lại của message độc. Read limit
	// decompressed phía app là 10MB (websocketx) — 16MB dư cho cả payload
	// không nén được (stored block phồng nhẹ hơn gốc).
	maxFlateLeftoverCorrupt = 1 << 24
)

// drainFlateLeftover — PATCH FORK (Zalo): đọc bỏ phần còn lại của message nén
// sau khi inflater đã kết thúc HOẶC báo corrupt (xem 2 điểm gọi trong mr.Read).
// Đọc qua mr.flateBufio để tiêu thụ CẢ byte đã buffer lẫn phần còn trên wire
// (qua mr.readFunc — gồm cả header continuation/FIN frame, nhờ đó mr.fin được
// cập nhật đúng trước message kế tiếp). Chặn trần theo từng đường gọi để
// stream desync hoặc độc hại không kéo drain vô hạn.
func (mr *msgReader) drainFlateLeftover(maxLeftover int) error {
	var (
		buf   [512]byte
		total int
	)
	for {
		n, err := mr.flateBufio.Read(buf[:])
		total += n
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if total > maxLeftover {
			err := fmt.Errorf("permessage-deflate message has more than %d leftover bytes after end of deflate stream", maxLeftover)
			mr.c.writeError(StatusProtocolError, err)
			return err
		}
	}
}

func (mr *msgReader) read(p []byte) (int, error) {
	for {
		if mr.payloadLength == 0 {
			if mr.fin {
				if mr.flate {
					return mr.flateTail.Read(p)
				}
				return 0, io.EOF
			}

			h, err := mr.c.readLoop(mr.ctx)
			if err != nil {
				return 0, err
			}
			if h.opcode != opContinuation {
				// PATCH FORK (Zalo): gắn ErrProtocolDesync (xem chú thích sentinel).
				err := desync(errors.New("received new data message without finishing the previous message"))
				mr.c.writeError(StatusProtocolError, err)
				return 0, err
			}
			mr.setFrame(h)

			continue
		}

		if int64(len(p)) > mr.payloadLength {
			p = p[:mr.payloadLength]
		}

		n, err := mr.c.readFramePayload(mr.ctx, p)
		if err != nil {
			return n, err
		}

		mr.payloadLength -= int64(n)

		if !mr.c.client {
			mr.maskKey = mask(p, mr.maskKey)
		}

		return n, nil
	}
}

type limitReader struct {
	c     *Conn
	r     io.Reader
	limit xsync.Int64
	n     int64
}

func newLimitReader(c *Conn, r io.Reader, limit int64) *limitReader {
	lr := &limitReader{
		c: c,
	}
	lr.limit.Store(limit)
	lr.reset(r)
	return lr
}

func (lr *limitReader) reset(r io.Reader) {
	lr.n = lr.limit.Load()
	lr.r = r
}

func (lr *limitReader) Read(p []byte) (int, error) {
	if lr.n < 0 {
		return lr.r.Read(p)
	}

	if lr.n == 0 {
		err := fmt.Errorf("read limited at %v bytes", lr.limit.Load())
		lr.c.writeError(StatusMessageTooBig, err)
		return 0, err
	}

	if int64(len(p)) > lr.n {
		p = p[:lr.n]
	}
	n, err := lr.r.Read(p)
	lr.n -= int64(n)
	if lr.n < 0 {
		lr.n = 0
	}
	return n, err
}
