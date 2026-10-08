---
name: zca-go-maintainer
description: >-
  Duy trì fork Go Zalo SDK theo hành vi zca-js: đối chiếu upstream, sửa login,
  cookie, listener và API, hoặc tích hợp SDK vào adapter Zalo cá nhân.
  Dùng cho thư viện này; không áp sang Zalo OA hoặc sao chép backend CRM.
---

# Duy trì SDK Zalo Go

Đây là SDK không chính thức cho Zalo cá nhân. Skill là hướng dẫn làm việc, không thay thư viện hoặc cấp quyền login/publish. Repo thư viện tách khỏi CRM; dependency SDK không chứa tenant, portal accounts, database, bot AI hay phân quyền CRM.

## Tìm nguồn đúng

Đọc go.mod, README, SOURCE-PROVENANCE.md, LICENSE và ATTRIBUTIONS.md tại repo trước khi sửa. Chữ ký Go lấy từ source, không đoán theo ví dụ JavaScript. Đọc [bản đồ nguồn](references/source-map.md) để vào đúng file. Không import module amrakk/zcago cũ hoặc dùng absolute replace sang máy cá nhân.

Upstream đối chiếu: https://github.com/RFS-ADRENO/zca-js ; docs: https://tdung.gitbook.io/zca-js (README upstream có thể trỏ site docs mới). Pin commit đang đọc trong ghi chép parity. Không claim đã đồng bộ toàn bộ vì cùng tên hàm hoặc số endpoint.

## Đồng bộ một thay đổi upstream

1. Chọn đúng diff upstream của API/protocol liên quan; kiểm endpoint, method, query defaults, payload, encryption, response và error semantics. Không auto-merge/auto-translate TypeScript.
2. Tìm counterpart Go bằng symbol/service/endpoint. Kiểm callers và wire types trước sửa, đặc biệt ID string/number và media upload/URL semantics.
3. Viết test RED dùng fixture giả/httptest/pure decode, không tự login account thật. Sửa phần nhỏ nhất để GREEN, kiểm race/cancellation và lỗi.
4. Giữ các transport patches được ghi trong ATTRIBUTIONS; rebase vendored websocket không được xoá ForceDeflate/Zalo handshake fixes.
5. Ghi upstream commit, Go files, test command/result và giới hạn chưa hỗ trợ. Lệnh benchmark/real-account chưa chạy phải ghi chưa đo/chưa kiểm.
6. Chạy build/vet/race phù hợp và review độc lập theo luật dự án. Publish/tag chỉ khi có quyền của user, không tự force push/merge hoặc tạo lịch đồng bộ tự động.

## Login và listener invariants

- Cookie/IMEI/UserAgent/language là saved session; cookie/QR/proxy credential không được log hoặc commit. LoginQR tạo session, Login khôi phục; UID phải khớp connection kỳ vọng trước bind instance.
- Một account có một web listener; lifecycle Start/Stop/context và disconnected/closed/error phải rõ. Không retry vô hạn hai tầng hoặc giữ goroutine cũ sau relogin.
- Drain Message và OldMessages; latter là replay/bù, không phải history UI có thể bỏ. External IDs vẫn strings, group và direct ThreadType khác nhau.
- Self-message phân biệt echo của send với manual từ điện thoại; SDK không quyết định CRM unread/auto reply. Integration layer chịu dedupe/correlation.
- Inspect emit/buffer policy: fork có drop khi đầy. Durable journal phía app chỉ bảo vệ sau enqueue commit, không bảo vệ event đã rơi trong SDK. Không gọi tăng buffer là no-loss guarantee.
- Delivered/Seen/Undo events khác read watermark CRM. Send timeout không chứng minh chưa accept; capability lookup theo request key phải kiểm thật trước tuyên bố reconcile/exactly-once.

## Tích hợp vào multi-provider app

SDK tại platform adapter; service app giữ identity/permission, session persistence policy, external contact/room resolution và journal orchestration. Không để adapter gọi SQL trực tiếp hoặc copy backend Zalo-only vào core chat. SDK instance là process resource, không DB object.

Đọc `docs/USAGE.md` theo API cần dùng; `docs/API_REFERENCE.md` để xác nhận endpoint. `docs/TEST_COVERAGE.md` là ghi chép lịch sử nguồn, không là bằng chứng fork hiện tại đã chạy real-account. Dùng ports nhỏ phía consumer, không kéo interface toàn bộ SDK vào service.

## Kiểm chứng

Dùng giới hạn tài nguyên của box/dự án. Ví dụ: `GOMAXPROCS=2 go build -p 2 ./...`, `go vet -p 2 ./...`, `go test -race -p 2 -timeout 120s ./...`. Đọc output thực tế. Không dùng cookies từ repo/backend khác để tự smoke; real-account test cần scope/account được chỉ định.

## Mốc và release của repo này

Đọc UPSTREAM.json và RELEASING.md ở root. Baseline lịch sử54df45d803fad7397eca04b2753cc0a894dc6e86; observed HEAD không phải synced HEAD. verified_complete_parity_commit null nghĩa chưa chứng minh parity; không tự điền HEAD. SDK tag v0.1.0 độc lập npm/wire version; không retag release cũ.
