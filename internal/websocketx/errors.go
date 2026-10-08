package websocketx

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"syscall"

	websocket "github.com/luannt2002/zalo-api-unofficial/internal/coderws"
)

func (c *client) isFatalErr(err error) bool {
	if c.isFatalContextErr(err) {
		return true
	}

	if isContextErr(err) {
		return false
	}

	// msgReader của bản vendor kẹt giữa message (inflater dừng sớm hoặc lỗi
	// giải nén giữa message fragmented) ⇒ mọi Read() sau đều lỗi NGAY mà không
	// đọc thêm byte nào — không thể tự hồi phục. Nếu coi là non-fatal, readLoop
	// retry liên tục tạo vòng lặp lỗi dày đặc nhiều dòng/ms (đúng signature
	// production). Phải teardown để listener reconnect sạch.
	if errors.Is(err, websocket.ErrPreviousMessageNotRead) {
		return true
	}

	// Stream byte đã lệch khỏi ranh giới frame (header rác: rsv lạ, opcode lạ,
	// control frame fragmented — đúng burst production vòng 3). Mọi header đọc
	// tiếp đều là rác, bản vendor cũng đã gửi close ProtocolError — retry chỉ
	// tạo vòng lặp lỗi dày đặc cùng-mili-giây. Teardown ngay từ lỗi đầu tiên.
	//
	// LƯU Ý: lỗi flate thuần ("dropped undecodable permessage-deflate message",
	// chuỗi chứa flate.CorruptInputError) KHÔNG nằm trong nhóm này — bản vendor
	// đã drain message độc và giữ framing đồng bộ, chỉ cần log rồi đọc tiếp.
	if errors.Is(err, websocket.ErrProtocolDesync) {
		return true
	}

	return isCloseErr(err) || isEOForUnexpected(err) || isConnClosedOrReset(err)
}

func (c *client) isFatalContextErr(err error) bool {
	if !isContextErr(err) {
		return false
	}

	return c.connCtx.Err() != nil
}

func closeInfoFromErr(err error) CloseInfo {
	var ci CloseInfo
	var ce websocket.CloseError
	if errors.As(err, &ce) {
		ci.Code = int(ce.Code)
		ci.Reason = ce.Reason
	} else {
		ci.Code = int(websocket.StatusInternalError)
		ci.Err = err
	}
	return ci
}

func isCloseErr(err error) bool {
	var ce websocket.CloseError
	return errors.As(err, &ce)
}

func isEOForUnexpected(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
}

func isConnClosedOrReset(err error) bool {
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	var op *net.OpError
	if errors.As(err, &op) {
		if se, ok := op.Err.(syscall.Errno); ok {
			if se == syscall.EPIPE || se == syscall.ECONNRESET {
				return true
			}
		}
	}

	return strings.Contains(err.Error(), "use of closed network connection")
}

func isContextErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
