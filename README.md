# Zalo API Unofficial — Go

**Bản Go triển khai lại hành vi/API của [zca-js](https://github.com/RFS-ADRENO/zca-js)** dành cho Zalo Web tài khoản cá nhân, không phải Zalo OA hoặc SDK chính thức của Zalo. Kế thừa mã Go zcago của Amrakk và các thay đổi trong fork `zalo-be-library`; duy trì độc lập bởi luannt2002. Giữ [LICENSE](LICENSE) và [ATTRIBUTIONS](ATTRIBUTIONS.md), không nhận toàn bộ source là tự viết mới.

## Cài và import

```sh
go get github.com/luannt2002/zalo-api-unofficial@v0.1.0
```

```go
import zcago "github.com/luannt2002/zalo-api-unofficial"
```

Version ban đầu `v0.1.0` là release thử nghiệm của bản tách; root package vẫn `zcago`. Không cần copy SDK hoặc absolute replace. [Hướng dẫn API Go](docs/USAGE.md) · [Tra cứu API](docs/API_REFERENCE.md).

## Mốc zca-js và parity

Mốc lịch sử trong inventory nguồn: [`54df45d803fad7397eca04b2753cc0a894dc6e86`](https://github.com/RFS-ADRENO/zca-js/commit/54df45d803fad7397eca04b2753cc0a894dc6e86), commit ngày12/04/2026. Upstream HEAD quan sát 28/10/2026 là [`0586dc5e4981985c5f16b9b3a5f285a02feb6f9b`](https://github.com/RFS-ADRENO/zca-js/commit/0586dc5e4981985c5f16b9b3a5f285a02feb6f9b) (merge PR #381, 13/10/2026); listener đã audit tới mốc này, xem `listener_audit` trong UPSTREAM.json.

**Không khẳng định toàn fork đã đồng bộ đầy đủ tới baseline hoặc HEAD:** file baseline riêng hiện thiếu, inventory là chứng cứ lịch sử và test parity toàn diện chưa có. Các thay đổi sau baseline phải audit tiếp. [UPSTREAM.json](UPSTREAM.json) phân biệt documented baseline, HEAD quan sát và verified parity (hiện null). [Provenance](SOURCE-PROVENANCE.md).

## Login/listener và tích hợp

NewZalo → Login cookie/IMEI/UserAgent hoặc LoginQR → kiểm own UID → lưu session → Listener.Start → nhận Message/OldMessages/Undo/receipt/lifecycle → Stop khi context huỷ. Dùng source để xác nhận chữ ký Go, không đoán theo JS.

Một account có một web listener; SDK buffers có khả năng drop khi đầy. Không claim no-loss/checkpoint/exactly-once. Cookie/QR/proxy credentials không log/commit; không dùng account thật tự động. App CRM giữ quyền/tenant, external room/contact resolution và journal, SDK không chứa SQL/portal/AI business logic.

## Duy trì và version

Đọc [RELEASING.md](RELEASING.md) để audit upstream, tạo tag và cập nhật consumer. Version Go SDK độc lập npm zca-js và wire APIVersion; không sửa wire constants để đánh release. [Skill](skills/zca-go-maintainer/SKILL.md) hướng dẫn agent, không nhúng SDK binary vào skill.

[Docs TEST_COVERAGE](docs/TEST_COVERAGE.md) là ghi chép lịch sử nguồn, không chứng nhận real-account/parity của release mới. [Báo cáo kiểm bản tách](EXTRACTION-REPORT.md). Sử dụng API không chính thức có thể khiến tài khoản bị hạn chế theo cảnh báo upstream.
