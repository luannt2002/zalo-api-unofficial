//go:build !js
// +build !js

// VENDORED FORK — nguồn: github.com/coder/websocket v1.8.12 (ISC License,
// xem LICENSE.txt cùng thư mục và ATTRIBUTIONS.md ở gốc repo).
//
// Lý do vendor: server WebSocket của Zalo nén permessage-deflate VÔ ĐIỀU KIỆN
// nhưng không echo Sec-WebSocket-Extensions trong handshake ⇒ mọi lib Go tuân
// RFC 7692 đều từ chối frame RSV1 và listener chết vòng lặp. Bản vendor này
// vá 4 điểm ở tầng transport (đánh dấu "PATCH FORK (Zalo)"):
//  1. dial.go/handshakeRequest — offer extension giống hệt Chrome:
//     "permessage-deflate; client_max_window_bits".
//  2. dial.go/dial + DialOptions.ForceDeflate — ép bật flate chiều ĐỌC khi
//     server không echo extension (chiều GỬI giữ nguyên không nén,
//     conn.flateWriteDisabled).
//  3. close.go/waitCloseHandshake + drainPayload — drain payload khi close
//     phải chịu read-timeout; upstream đọc thẳng bufio nên Close() treo vô
//     hạn khi stream desync (hệ quả trực tiếp của lỗi RSV1 ở trên).
//  4. read.go/msgReader.Read + drainFlateLeftover — server Zalo kết thúc
//     deflate stream bằng block BFINAL=1 (Z_FINISH) thay vì sync-flush + strip
//     đuôi theo RFC 7692 §7.2.1 ⇒ inflater EOF sớm khi message còn phần thừa
//     (padding/đuôi 00 00 ff ff/FIN frame rỗng của message fragmented)
//     ⇒ msgReader.fin kẹt false, mọi Read sau lỗi "previous message not read
//     to completion" lặp vô hạn. Fix: drain nốt phần còn lại của message khi
//     flate EOF (đúng hành vi browser: bỏ qua byte thừa sau cuối stream).
//     Kèm sentinel read.go/ErrPreviousMessageNotRead cho tầng trên nhận diện
//     trạng thái kẹt không hồi phục được.
//  5. read.go/msgReader.Read — cô lập lỗi flate theo TỪNG message. Production
//     vòng 3: "failed to read: flate: corrupt input before offset 8" rồi burst
//     "opPong: received fragmented control frame" + rsv đủ tổ hợp cùng 1ms,
//     close 1011, lặp ~47 lần/3 phút. Corrupt ngay offset <8 = byte đầu message
//     không phải đầu deflate stream hợp lệ (dưới burst, edge Zalo cắt ranh giới
//     WS message lệch ranh giới stream nén). Bug cũ: abort giữa message ⇒ byte
//     nén còn lại kẹt trong flateBufio/wire ⇒ Reader() kế đọc header giữa
//     payload ⇒ cascade desync ⇒ reconnect ⇒ server gửi lại đúng batch độc ⇒
//     vòng lặp. Fix: lỗi flate thuần (CorruptInputError/InternalError) thì
//     drain phần còn lại ở tầng framing (cơ chế patch 4) và chỉ DROP message
//     đó — connection sống tiếp, message thẳng hàng kế decode bình thường.
//     LƯU Ý chẩn đoán bị BÁC BỎ khi điều tra: không phải "giữ state inflater
//     giữa các message" — resetFlate() vốn Reset inflater sạch mỗi message;
//     và KHÔNG được chuyển ForceDeflate sang serverNoContextTakeover: chiều
//     đọc, dictionary chỉ NỚI LỎNG kiểm tra distance (superset) — bỏ dict
//     không sửa được corrupt input mà còn vỡ trường hợp server context
//     takeover thật (guard: TestForceDeflateContextTakeoverFragmentedStream).
//  6. read.go/ErrProtocolDesync — sentinel gắn vào các lỗi header rác (rsv lạ,
//     opcode lạ, control frame fragmented/quá dài, continuation sai chỗ, close
//     payload rác; giữ nguyên text upstream qua desyncError). Tầng
//     internal/websocketx coi là FATAL: teardown + reconnect ngay lỗi đầu
//     tiên thay vì retry tạo burst như production log.
//
// Đã lược phần wasm (ws_js), wsjson, test/autobahn/example của upstream.
// Khi re-baseline theo upstream TUYỆT ĐỐI không revert các patch trên.
//
// Package websocket implements the RFC 6455 WebSocket protocol.
//
// https://tools.ietf.org/html/rfc6455
//
// Use Dial to dial a WebSocket server.
//
// Use Accept to accept a WebSocket client.
//
// Conn represents the resulting WebSocket connection.
//
// The examples are the best way to understand how to correctly use the library.
//
// The wsjson subpackage contain helpers for JSON and protobuf messages.
//
// More documentation at https://github.com/coder/websocket.
//
// # Wasm
//
// The client side supports compiling to Wasm.
// It wraps the WebSocket browser API.
//
// See https://developer.mozilla.org/en-US/docs/Web/API/WebSocket
//
// Some important caveats to be aware of:
//
//   - Accept always errors out
//   - Conn.Ping is no-op
//   - Conn.CloseNow is Close(StatusGoingAway, "")
//   - HTTPClient, HTTPHeader and CompressionMode in DialOptions are no-op
//   - *http.Response from Dial is &http.Response{} with a 101 status code on success
package websocket
