# Bản đồ đọc source SDK

| Việc | Nguồn Go |
|---|---|
| Factory/login | zcago.go, credentials.go, options.go, session/auth |
| Public API signatures | apis.go; implementation api/*.go |
| Request/session/crypto | session/ và internal/ theo imports thực tế |
| Realtime lifecycle | listener/listener.go, retry.go |
| Message/event decoding | listener/router.go, model/message.go và event models |
| Buffers/drop | listener/channels.go, emit |
| Send | api/send_message.go; tìm SendMessage symbol trong api/ nếu source đổi |
| Transport patches | ATTRIBUTIONS.md, internal/coderws/ |
| Fork usage | docs/USAGE.md, docs/API_REFERENCE.md |

Không copy CRM Manager/BotLogin/services/repository từ dự án nguồn. Chỉ tham khảo cách gọi API khi cần hiểu lifetime/cookie/Own UID; không mở datastore/cookie files.

Khi đối chiếu zca-js, đọc src cùng test và commit diff API cần sửa. Chưa xác định counterpart thì ghi rõ chưa map, không invent tên method. Test phải so protocol output/error behavior, không chỉ Assert tên hàm tồn tại.
