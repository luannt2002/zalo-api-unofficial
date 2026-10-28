# Changelog

## Unreleased (đề xuất v0.2.0)

Đồng bộ zca-js tới `0586dc5` (2026-10-28) + bản vá cộng đồng zca-js 2.1.2.

Listener và model đã đối chiếu tới commit đó, kèm các PR còn mở chạm `listen.ts` (#380, #387, #391, một phần
#371); điểm bắt đầu lần đồng bộ sau là `listener_synced_through` trong `UPSTREAM.json`, chi tiết từng PR và bảng
phủ sự kiện ở `listener_audit`.

### Thêm (tương thích, nhưng `listener.Listener` có thêm method — ai tự cài interface này phải bổ sung)
- Định tuyến cmd 551 như 501 (upstream PR #378).
- `UnreadCleared()` cho cmd 504/524 (upstream PR #380); `PinConversations()`, `UnreadMark()`, `Voip()` cho cmd 601.
- Thu hồi nằm trong lô tin cũ (510/511, phát lại sau khi nối lại) phát ra `Undo()` sau `OldMessages()` (upstream PR #391, đang mở).
- `Unhandled()` + `UnhandledFrames()`: khung/control router chưa xử lý.
- `DroppedOptionalEvents()`: tin rơi ở kênh tuỳ chọn (xem "Đổi hành vi").
- `DroppedErrors()`: lỗi bị bỏ vì kênh `Error()` (buffer 16) đầy — trước đây rơi không đếm.
- `DecodeFailures()` + `*DecodeError{Op, MsgID, Raw, Err}`: tin không giải mã được không còn biến mất im lặng.
- `events.*`: trường `Errors` / `Err` / `RawData` / `Content` mới; trường cũ giữ nguyên.

### Đổi hành vi
- Giải mã từng tin trong lô: một tin hỏng chỉ mất tin đó (trước: mất cả lô hoặc mất im lặng).
- Trường vô hướng nhận cả số lẫn chuỗi (vd `quote.ownerId`); quote thiếu `ownerId` giờ là `""` thay vì `"0"`.
  Trường số nhận `""` / chỉ khoảng trắng thành `0` (như `Number("")` của JS); chuỗi không phải số vẫn báo lỗi.
- Mất TCP không khung đóng báo `1006` thay vì `1011`.
- `1006` không có lịch từ máy chủ: tối đa 8 lần (2/5/10/20/30/60 s) rồi `Closed()` (trước: thử lại vô hạn).
  Ứng dụng phải xử lý `Closed()` (đăng nhập lại).
- Bộ đếm thử lại chỉ về 0 sau 30 s kết nối ổn định (trước: ngay khi nhận khoá).
- `Stop()` trong lúc chờ thử lại huỷ lần nối lại.
- Nối lại mà quay số lỗi (mạng còn đứt, máy chủ từ chối nâng cấp) tính như một lần đóng `1006`: mỗi lượt phát
  `Error()` + `Disconnected()` rồi đi tiếp lịch thử lại; chỉ `Closed()` khi hết lượt (trước: `Closed()` ngay lượt
  quay số lỗi đầu tiên, nên lịch 2/5/10/20/30/60 s × 8 thực tế không chạy khi mạng chưa về).
  Mạng/máy chủ hỏng hẳn: `Closed()` tới sau khoảng 2–4 phút (8 lượt 2+5+10+20+30+60+60+60 s, mỗi lượt dao động
  [d/2, d], cộng thời gian quay số), không còn tới ngay.
- `Stop()` rồi `Start()` lại trên kết nối đang sống chạy được (trước: thường báo "Already started" vì client cũ
  không được dọn). `Stop()` huỷ và đọc client trong cùng một vùng khoá, nên không còn cửa sổ để lần nối lại
  đang quay số tạo ra một kết nối không ai đóng.
- Lượt nối lại quay số lỗi mà xong muộn không còn xoá client / huỷ context của một `Start()` mới hơn (hồi quy của
  chính đợt này: `Start()` trả `nil` mà listener đã chết). Bước reset / `Closed` / huỷ khi đóng kết nối chỉ chạy
  nếu context của lượt đó vẫn là lượt mới nhất.
- `ZCAError.Error()` in kèm nguyên nhân (cắt 512 byte).
- Bảo mật: lỗi quay số WebSocket (`*url.Error`) không còn in query của URL ws (mang `zpw_sek` và tham số phiên)
  — giữ scheme+host+path, query thành `?<redacted>`; log "Rotating endpoint" cũng bỏ query. Lưu ý
  `DecodeError.Error()` vẫn chứa tới 512 byte nội dung tin (dữ liệu người dùng) — xem USAGE.
- Friend action chưa có decoder đi vào `Unhandled()`; control nhóm/bạn bè giải hỏng phát `*DecodeError`.
- Tin cũ của nhóm (cmd 511) là `model.GroupMessage` (ThreadID = nhóm), như upstream; trước là `UserMessage` với
  ThreadID = người gửi — ứng dụng nào đang tự sửa thread = `idTo` phải bỏ phần sửa đó. Code ép kiểu `m.(model.UserMessage)` trên
  phần tử của lô 511 giờ sẽ panic (dạng một giá trị) hoặc trả `ok=false` — dùng type switch.
- `OldMessages` / `OldReactions` lấy loại thread theo cmd (510/610 = 1-1, 511/611 = nhóm), không đoán theo payload
  (trước: lô nhóm rỗng bị báo là 1-1). Thu hồi không còn lẫn trong `OldMessages.Messages`. Lô tin cũ chỉ toàn thu
  hồi phát `OldMessages()` rỗng rồi các `Undo()` (như upstream PR #391).
- Item `chat.undo` không giải được (vd `content.cliMsgId` không phải số) phát `*DecodeError`; trước đây rơi sang
  nhánh tin và hiện ra như một tin mới.
- Typing (cmd 602) giải khoan dung như tin (vd `isPC` dạng chuỗi, `gid` dạng số); không giải được thì phát
  `*DecodeError` thay vì một typing rỗng (UID `""`). Hành động 602 khác typing / typing lạ đi vào `Unhandled()`
  (trước: bỏ im lặng).
- `events.OldMessagesEventData` đổi kiểu trường (giải từng tin như tin sống) và bỏ `Errors`; gói `events` là
  phần giải mã nội bộ của listener.
- Kênh tuỳ chọn (`Unhandled`, `UnreadCleared`, `PinConversations`, `UnreadMark`, `Voip`) đầy thì bỏ tin MỚI và
  đếm vào `DroppedOptionalEvents()`, không vào `DroppedEvents()` — ứng dụng không đọc các kênh này không bị
  báo động giả. `DroppedEvents()` giờ chỉ là kênh lõi (tin, reaction, undo, nhóm, bạn bè, receipt...).

### Đã biết (chưa sửa trong bản này)
- `Stop()` có thể chờ tới khi lượt quay số đang chạy kết thúc hoặc hết hạn: lượt nối lại giữ `ln.mu` trong lúc quay số.
- Lỗi REST trong `internal/httpx` chưa được che query của URL như lỗi quay số WebSocket.
- Huỷ context đã truyền cho `Start()` không đóng socket (đo: máy chủ còn thấy kết nối mở sau 8 s, `client` kẹt,
  `Start()` sau đó báo "Already started"); có từ trước đợt này. Dùng `Stop()` để dừng.
- Sau `Stop()`, `Disconnected()` / `Closed()` có thể tới hoặc không (đo 3/40 lần có `Closed`); zca-js luôn phát
  `closed(1000)`. Chưa chốt hướng.
