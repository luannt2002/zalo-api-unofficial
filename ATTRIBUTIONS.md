# Attributions — mã nguồn bên thứ ba

## github.com/coder/websocket v1.8.12 (vendored tại `internal/coderws/`)

- **Tác giả:** Anmol Sethi và các contributor của coder/websocket.
- **License:** ISC — giữ nguyên văn tại `internal/coderws/LICENSE.txt`.
- **Phiên bản gốc:** v1.8.12.
- **Lý do vendor thay vì require:** server WebSocket của Zalo nén
  permessage-deflate vô điều kiện nhưng không echo `Sec-WebSocket-Extensions`
  trong response 101; lib tuân RFC 7692 từ chối frame RSV1 ⇒ listener chết
  vòng lặp reconnect. Phải vá ở tầng transport, không thể vá từ bên ngoài API.
- **Khác biệt so với upstream** (đều đánh dấu comment `PATCH FORK (Zalo)`):
  1. `dial.go` — offer extension giống hệt Chrome
     (`permessage-deflate; client_max_window_bits`) và verify chấp nhận server
     echo `client_max_window_bits`/`=15`.
  2. `dial.go`, `conn.go`, `write.go` — thêm `DialOptions.ForceDeflate`: nếu
     server không echo extension thì vẫn bật flate chiều đọc (context-takeover,
     đúng semantics per-message RSV1 của RFC 7692); chiều gửi không nén
     (`conn.flateWriteDisabled`).
  3. `close.go` — `waitCloseHandshake` drain payload qua `drainPayload`
     (chịu read-timeout) thay vì đọc thẳng bufio; tránh `Close()` treo vô hạn
     khi stream đã desync.
  4. Lược bỏ phần wasm (`ws_js*`, `internal/wsjs`), `wsjson`, test/autobahn/
     example/CI của upstream; đổi import path nội bộ về
     `github.com/luannt2002/zalo-api-unofficial/internal/coderws/...`.
- **Lưu ý re-baseline:** khi nâng phiên bản theo upstream, PHẢI giữ lại 2 patch
  wire-protocol ở trên (xem header `internal/coderws/doc.go`).
