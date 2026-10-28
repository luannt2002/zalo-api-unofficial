# Changelog

## Unreleased (đề xuất v0.2.0)

Đối chiếu zca-js tới `0586dc5` và zca-js 2.1.2 community patch; chi tiết trong `UPSTREAM.json` (`listener_audit`).

### Thêm (tương thích, nhưng `listener.Listener` có thêm method — ai tự cài interface này phải bổ sung)
- Định tuyến cmd 551 như 501 (upstream PR #378).
- `UnreadCleared()` cho cmd 504/524 (upstream PR #380); `PinConversations()`, `UnreadMark()`, `Voip()` cho cmd 601.
- `Unhandled()` + `UnhandledFrames()`: khung/control router chưa xử lý.
- `DecodeFailures()` + `*DecodeError{Op, MsgID, Raw, Err}`: tin không giải mã được không còn biến mất im lặng.
- `events.*`: trường `Errors` / `Err` / `RawData` / `Content` mới; trường cũ giữ nguyên.

### Đổi hành vi
- Giải mã từng tin trong lô: một tin hỏng chỉ mất tin đó (trước: mất cả lô hoặc mất im lặng).
- Trường vô hướng nhận cả số lẫn chuỗi (vd `quote.ownerId`); quote thiếu `ownerId` giờ là `""` thay vì `"0"`.
- Mất TCP không khung đóng báo `1006` thay vì `1011`.
- `1006` không có lịch từ máy chủ: tối đa 8 lần (2/5/10/20/30/60 s) rồi `Closed()` (trước: thử lại vô hạn).
  Ứng dụng phải xử lý `Closed()` (đăng nhập lại).
- Bộ đếm thử lại chỉ về 0 sau 30 s kết nối ổn định (trước: ngay khi nhận khoá).
- `Stop()` trong lúc chờ thử lại huỷ lần nối lại.
- `ZCAError.Error()` in kèm nguyên nhân (cắt 512 byte).
- Friend action chưa có decoder đi vào `Unhandled()`; control nhóm/bạn bè giải hỏng phát `*DecodeError`.
- Kênh mới không ai đọc vẫn tính rơi vào `DroppedEvents()` (trừ `Unhandled()`).
