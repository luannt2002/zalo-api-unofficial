package websocketx

// Test nội bộ (cùng package) cho phân loại lỗi fatal của readLoop — đặc biệt
// lỗi "previous message not read to completion" từ bản vendor: msgReader kẹt
// giữa message thì retry vô nghĩa, phải fatal để teardown + reconnect, tránh
// vòng lặp lỗi dày đặc như production log.

import (
	"compress/flate"
	"context"
	"errors"
	"fmt"
	"testing"

	websocket "github.com/luannt2002/zalo-api-unofficial/internal/coderws"
)

func TestPreviousMessageNotReadIsFatal(t *testing.T) {
	t.Parallel()

	c := &client{connCtx: context.Background()}

	// Đúng dạng lỗi thật: reader() wrap sentinel qua errd.Wrap ("failed to get
	// reader: %w") rồi readLoop nhận nguyên chuỗi wrap đó.
	err := fmt.Errorf("failed to get reader: %w", websocket.ErrPreviousMessageNotRead)
	if !c.isFatalErr(err) {
		t.Fatal("ErrPreviousMessageNotRead phai la fatal de teardown + reconnect, khong duoc retry spin")
	}

	// Lỗi thường khác vẫn phải non-fatal (giữ semantics cũ của readLoop).
	if c.isFatalErr(errors.New("failed to read: corrupt input")) {
		t.Fatal("loi doc thuong (khong sentinel) van phai non-fatal")
	}
}

// TestProtocolDesyncIsFatal — production vòng 3: sau điểm desync mọi header
// đều rác ("opPong: received fragmented control frame", rsv đủ tổ hợp), retry
// chỉ tạo burst lỗi cùng-mili-giây. Sentinel ErrProtocolDesync từ bản vendor
// phải FATAL để teardown + reconnect ngay lỗi đầu tiên.
func TestProtocolDesyncIsFatal(t *testing.T) {
	t.Parallel()

	c := &client{connCtx: context.Background()}

	// Đúng dạng wrap thật: handleControl → readLoop → reader → errd.Wrap.
	err := fmt.Errorf("failed to get reader: failed to handle control frame opPong: %w", websocket.ErrProtocolDesync)
	if !c.isFatalErr(err) {
		t.Fatal("ErrProtocolDesync phai la fatal de teardown + reconnect, khong duoc retry spin")
	}
}

// TestDroppedCorruptDeflateMessageIsNotFatal — PATCH FORK số 5 của bản vendor:
// message nén độc (corrupt input — kiểu "flate: corrupt input before offset 8"
// production) đã được drain ở tầng framing và DROP đúng 1 message; connection
// còn đồng bộ nên readLoop chỉ log rồi đọc tiếp, KHÔNG teardown.
func TestDroppedCorruptDeflateMessageIsNotFatal(t *testing.T) {
	t.Parallel()

	c := &client{connCtx: context.Background()}

	err := fmt.Errorf("failed to read: dropped undecodable permessage-deflate message (frame stream resynced): %w", flate.CorruptInputError(8))
	if c.isFatalErr(err) {
		t.Fatal("loi flate da duoc co lap theo message phai non-fatal — teardown o day la tai tao reconnect storm")
	}
}
