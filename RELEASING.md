# Version và đồng bộ tiếp

## Ba mốc khác nhau

- Tag `v0.1.0`: version SDK Go của repo này; initial experimental, không phải npm version.
- `last_documented_baseline` trong UPSTREAM.json: mốc upstream có chứng cứ inventory; chưa là proof parity.
- `listener_synced_through` trong UPSTREAM.json: commit upstream mà listener + model đã đối chiếu XONG (kèm PR đã port / cố ý hoãn). Đây là điểm bắt đầu diff của lần đồng bộ sau.
- `upstream_head_observed`: HEAD đã nhìn thấy, không tự là commit đã port. `verified_complete_parity_commit` chỉ cập nhật khi audit/test đủ, hiện null.

## Đồng bộ một lượt (listener / model)

Bắt đầu từ `listener_synced_through.commit`, không từ baseline cũ `54df45d`. Chạy ở gốc repo này; upstream nằm
trong một checkout riêng ngoài repo (`$UP` dưới đây).

```sh
UP=../zca-js-upstream                                   # checkout riêng, chỉ đọc
[ -d "$UP" ] || git clone https://github.com/RFS-ADRENO/zca-js "$UP"
git -C "$UP" fetch origin
FROM=$(jq -r .listener_synced_through.commit UPSTREAM.json)
TO=$(git -C "$UP" rev-parse origin/main)

# 1. Commit mới chạm listener/model/giải mã
git -C "$UP" log --oneline "$FROM..$TO" -- src/apis/listen.ts src/models src/utils.ts
git -C "$UP" diff "$FROM..$TO" -- src/apis/listen.ts src/models src/utils.ts
# toàn bộ commit trong khoảng, để phân loại phần ngoài listener (deferred / not-applicable)
git -C "$UP" log --oneline "$FROM..$TO"

# 2. PR còn mở chạm listen.ts (so với listener_audit.open_prs_reviewed)
curl -s 'https://api.github.com/repos/RFS-ADRENO/zca-js/pulls?state=open&per_page=100' \
  | jq -r '.[] | "\(.number) \(.head.sha[0:7]) \(.updated_at) \(.title)"'
curl -s https://api.github.com/repos/RFS-ADRENO/zca-js/pulls/<N>/files | jq -r '.[].filename'
curl -s -H 'Accept: application/vnd.github.v3.diff' https://api.github.com/repos/RFS-ADRENO/zca-js/pulls/<N>
```

3. Port từng thay đổi theo nếp **test đỏ trước rồi xanh**: viết test Go tái hiện hành vi upstream (khung giả qua
   `router`, máy chủ WebSocket giả cho vòng đời), chạy thấy đỏ trên code cũ, sửa code, thấy xanh. Thay đổi nào Go
   đã đúng sẵn thì ghi "xanh ngay", không bịa đỏ. Review theo API/protocol, không chép TypeScript sang Go; giữ các
   bản vá websocket (PATCH FORK). Một thay đổi = một commit.
4. Kiểm cả gói: `GOMAXPROCS=2 go build -p 2 ./... && go vet -p 2 ./... && go test -p 2 ./... && go test -race -p 2 ./listener/...`.
   So kết quả test theo TÊN với lần chạy trước (không đếm số).
5. Cập nhật UPSTREAM.json: `listener_synced_through` (`commit` = `$TO` đã kiểm, `date`, `ported`, `deferred` kèm
   lý do), `upstream_head_observed`, `observed_on`, và `listener_audit` (từng commit/PR: ported / deferred /
   not-applicable + tên test). Bảng `listener_audit.event_coverage` phải còn khớp mọi `emit(...)` của listen.ts.
6. Mục CHANGELOG mới mở đầu bằng dòng `Đồng bộ zca-js tới <SHA ngắn> (<ngày>) + <bản vá cộng đồng nếu có>`, rồi liệt
   kê luồng/sự kiện mới và thay đổi hành vi ứng dụng phải biết.
7. Review độc lập, rồi tag phiên bản mới (mục dưới). Không tái dùng tag đã publish; không tự login account hay
   publish ngoài quyền user.

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
