# Version và đồng bộ tiếp

## Ba mốc khác nhau

- Tag `v0.1.0`: version SDK Go của repo này; initial experimental, không phải npm version.
- `last_documented_baseline` trong UPSTREAM.json: mốc upstream có chứng cứ inventory; chưa là proof parity.
- `upstream_head_observed`: HEAD đã nhìn thấy, không tự là commit đã port. `verified_complete_parity_commit` chỉ cập nhật khi audit/test đủ, hiện null.

## Đồng bộ một lượt

Clone/fetch RFS-ADRENO/zca-js vào checkout riêng. Diff từ documented baseline54df45d803fad7397eca04b2753cc0a894dc6e86 tới target commit; HEAD hiện quan sát là dadfef18bcac53537741855c99f084152da1fad9. Review theo API/protocol, không merge TypeScript vào Go. Mapping trạng thái từng thay đổi: supported/ported/tested/deferred/not-applicable; ghi SHA và tests. Không tăng mốc complete parity nếu còn thiếu phần không được giải thích.

Sửa endpoint/payload/response/error semantics trong Go, tests trước code; giữ websocket patches. Chạy GOMAXPROCS=2 go build -p 2 ./..., go vet -p 2 ./..., go test -race -p 2 -timeout 120s ./.... Review độc lập, ghi bằng chứng thật, không tự login account hoặc publish ngoài quyền user.

## Release Go

Sau commit/review, chọn version: patch cho fixes tương thích, minor cho API mới tương thích; v0 còn experimental và có thể breaking, phải ghi changelog. Từ v1 cần ổn định API; breaking v2 cần module/import suffix /v2 đúng Go semantics. Đừng squash lịch sử sau mỗi release; một root commit chỉ áp lần khởi tạo user yêu cầu.

```sh
git tag -a v0.1.0 -m "Initial Go SDK extraction; historical zca-js baseline recorded"
git push origin main
git push origin v0.1.0
```

Không di chuyển/reuse tag đã publish. Release sau dùng tag mới và ghi upstream target/evidence. Xem versions bằng `go list -m -versions github.com/luannt2002/zalo-api-unofficial`.

## Consumer app

```sh
go get github.com/luannt2002/zalo-api-unofficial@v0.1.0
# hoặc cập nhật tới release mới sau review:
go get github.com/luannt2002/zalo-api-unofficial@latest
go list -m github.com/luannt2002/zalo-api-unofficial
```

Import giữ nguyên github.com/luannt2002/zalo-api-unofficial; version chọn ở go.mod, không trong import (ngoại trừ major /v2). `@latest` resolve một version tại lúc chạy, app không tự đổi dependency khi upstream push. Commit go.mod/go.sum và chạy suite consumer; không auto upgrade production. Module chưa runtime import vẫn có thể bị tidy bỏ, không tạo wrapper giả để giữ dependency.
