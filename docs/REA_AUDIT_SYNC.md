# Báo Cáo Audit: Đồng Bộ Logic zca-js & Tích Hợp Giá Trị REA từ debug/ vào zalo-api-unofficial

> **Ngày thực hiện:** 05/11/2026  
> **Kho nguồn:** `/home/luannt/application/opensource/zca-js` & `/home/luannt/application/debug`  
> **Kho đích:** `/home/luannt/application/opensource/zalo-api-unofficial`  
> **Phương pháp luận:** REA (Reverse Engineer Anything) — Tri thức dựa trên bằng chứng nhị phân, đối chiếu chéo 3 nguồn (Web ↔ Mobile APK ↔ Native Ghidra), không suy đoán, chuẩn Golang idiomatic.

---

## 1. Bảng Phân Định Nguồn Gốc (Provenance Matrix)

Nhằm đảm bảo tính minh bạch khi bảo trì và review code, toàn bộ các thay đổi được phân chia rạch ròi thành 2 nhóm:

| Tính Năng / Thành Phần | Nguồn Gốc | File Golang Liên Quan | Tóm Tắt Trách Nhiệm |
|---|---|---|---|
| **Debug Trace System** | **Sync zca-js** | `session/options.go`<br>`options.go`<br>`internal/httpx/client.go` | Cổng quan sát telemetry theo thời gian thực (HTTP latency, status, WS frame/stage) không để lộ thông tin nhạy cảm. |
| **WebSocket Routing Parity** | **Sync zca-js** | `listener/router.go`<br>`listener/channels.go` | Định tuyến khung cmd 551 (tin nhắn 1-1 mở rộng), bắt frame unhandled chuẩn xác và cmd 620 (`ResetCloudEvent`). |
| **Identity Resolution (GID)** | **REA `debug/`** | `pkg/gid/*`<br>`api/encrypt_global_id.go`<br>`api/decrypt_global_id.go`<br>`api/convert_global_id.go` | Cấu trúc GID 32 ký tự base32hex, giải mã Snowflake epoch 2011, gọi endpoint `/api/gid/encrypt` (noiseUids) và `/api/gid/decrypt` bóc tách envelope đa lớp. |
| **Offline Viewer UID Translation** | **REA `debug/`** | `pkg/gid/mask.go`<br>`pkg/gid/store.go`<br>`api/translate_viewer_uid.go` | Công thức toán học chuyển đổi UID chéo giữa 2 bot hoàn toàn offline (0-network) qua XOR Mask của `send2me_id`. |
| **Media URL Renewal (E-X3r)** | **REA `debug/`** | `api/renew_link.go`<br>`listener/events/controls.go`<br>`listener/router.go` | Cơ chế 2 pha làm mới link ảnh/file hết hạn: HTTP POST cmd 12094 (`/api/message/renewlink`) + server-push WebSocket cmd 601 (`renew_link`). |
| **Send2Me ("Cloud của tôi")** | **REA `debug/`** | `session/context.go`<br>`session/context_impl.go`<br>`api/send2me.go` | Nhận diện hội thoại Cloud (`send2me_id`), phương thức `IsSend2me(id)`, và tự động kích hoạt `isGroup = 1` cho lưu trữ đám mây. |
| **Multi-Domain CookieJar RFC 6265** | **REA `debug/`** | `credentials.go`<br>`credentials_test.go` | Hỗ trợ parse cookie `key` từ `tough-cookie`, tự động nhóm cookie theo subdomain (`chat.zalo.me`, `id.zalo.me`, `zaloapp.com`) tránh bị Go drop cookie chéo domain. |
| **GetMiniProfiles (Kênh GID Member Thật)** | **REA `debug/`** | `api/get_mini_profiles.go`<br>`api/get_mini_profiles_test.go`<br>`apis.go` | Kênh lấy GID member thật (1 req / ≤100 uid, tự động batching), tránh bẫy echo-viewer của `/api/social/group/members`, chuẩn hóa `bizPkg` → `bizInfo`. |
| **Permanent Legacy Account ID (UID-9)** | **REA `debug/`** | `session/uid9.go`<br>`session/uid9_test.go`<br>`credentials.go`<br>`apis.go` | Trích xuất khóa tài khoản gốc 9 số vĩnh viễn xuyên phiên từ cookie `zpw_sek` và `zpsid` (`tag4.UID-9.selector.secret`). |
| **Test Fixtures Nạp Từ Môi Trường** | **Quy chuẩn kiểm thử** | `internal/testenv/env.go`<br>`.env.test` | Triệt tiêu hoàn toàn test ID hardcode inline; nạp và kiểm định toán học tự động các fixtures (`Send2me`, `UID`, `GID`, `UID9`). |
| **HarvestGroup (Thu Hoạch Nhóm 2 Bước)** | **REA `debug/`** | `api/harvest_group.go`<br>`api/harvest_group_test.go`<br>`apis.go` | Thu hoạch toàn bộ metadata nhóm và GID thật mọi member chỉ trong 2 network request (0 call tới `/api/gid/*`), hoàn toàn triệt tiêu nguy cơ bị rate limit. |
| **ResolveUIDViaGID (Cầu Nối Nhận Tin)** | **REA `debug/`** | `api/resolve_uid_via_gid.go`<br>`apis.go` | Dịch ngược GID nhận từ tài khoản khác sang UID của viewer hiện tại để gửi tin nhắn. |
| **Linear AES-256-GCM Media Decryptor** | **REA `debug/`** | `internal/cryptox/aes.go`<br>`internal/cryptox/aes_test.go` | Bộ giải mã media E2EE Zalo dùng linear AES-256-GCM (key 32B, IV 12B, Tag 16B). |
| **ZCID Transport Decoder** | **REA `debug/`** | `internal/httpx/encryption.go`<br>`internal/httpx/encryption_test.go` | Giải mã chuỗi `zcid` hex bằng `ZCID_KEY` để trích xuất `apiType`, `imei`, và `firstLaunchTime`. |
| **Bảo Vệ Phiên Khỏi Delete Markers** | **REA `debug/`** | `credentials.go`<br>`credentials_test.go` | Tự động loại bỏ cookie xóa (`EXPIRED`, exp-1970) khi nạp vào CookieJar tránh làm chết session thật. |
| **Mô Hình Hóa 160-bit GID & Cohort W** | **REA `debug/`** | `pkg/gid/snowflake.go`<br>`pkg/gid/snowflake_test.go` | Chuyển đổi 2 chiều Base32Hex ↔ 160-bit blob, trích xuất 19 bytes payload và 32-bit cohort field `W`. |
| **Dịch GroupID Giữa Các Viewer** | **REA `debug/`** | `pkg/gid/mask.go`<br>`pkg/gid/mask_test.go` | Chuyển đổi GroupID giữa các tài khoản qua cùng công thức mask XOR (Round 11d REA: GroupID cũng per-viewer). |
| **ZNoise Voicecall Obfuscation Engine** | **REA `debug/`** | `api/get_znoise.go`<br>`api/get_znoise_test.go` | Endpoint `/api/znoise` (cmd 12390) sinh UID số âm `int64` (bit 63 = 1) theo ngữ cảnh cuộc gọi nhóm. |
| **Native Socket Parameters Encoder** | **REA `debug/` (Ghidra)** | `internal/cryptox/socket.go`<br>`internal/cryptox/socket_test.go` | Tái tạo chính xác hàm `encodeSocketParamsNative` từ `libnative_utils.so` (AES-256-CBC, IV=0, Base64, rodata `const16` @0x18030). |
| **Live Cloud Group History** | **REA `debug/`** | `api/get_group_chat_history.go`<br>`api/get_group_chat_history_test.go` | Nâng cấp từ `/api/group/history` (đã chết) sang `/api/cm/getrecentv2` (cloud message live), hỗ trợ unmarshal dữ liệu stringified và bóc tách cursor. |
| **Media URL Classifier & Expiry Engine** | **REA `debug/`** | `pkg/media/classifier.go`<br>`pkg/media/classifier_test.go` | Nhận diện họ host Zalo CDN, bóc tách chữ ký HMAC, tính toán mốc hết hạn +60 ngày và thời gian còn lại. |
| **Mobile HTTP Key Derivation** | **REA `debug/`** | `internal/cryptox/zcid_key.go`<br>`internal/cryptox/zcid_key_test.go` | Phái sinh 32-byte HTTP key từ `zcid` hex theo hoán vị vị trí chẵn/lẻ-đảo-chiều native `CoreUtils.a`. |
| **Offline Session Diagnostics** | **REA `debug/`** | `session/inspect.go`<br>`session/inspect_test.go` | Thanh tra file session theo 5 quy tắc thực chiến REA: bóc UID-9 offline, tính ngày tạo viewer Snowflake, phát hiện delete marker. |
| **Phân Tích QR Error -20** | **REA `debug/`** | `docs/REA_AUDIT_SYNC.md` | Bản chất lỗi -20 do native Rust `libzproto_trust_ffi.so` (`zproto_trust_decode_qr`) khi client bị ép sang luồng `ntk` device-linking. |

---

## 2. Chi Tiết Phần A: Đồng Bộ Từ `zca-js` (Upstream Parity)

### 2.1. Debug Tracing (`WithDebugTrace`)
Trong `zca-js`, cơ chế `emitDebugTrace` được chèn vào các tầng transport của HTTP (`src/utils.ts`) và WebSocket (`src/apis/listen.ts`).
- **Triển khai Go:** Cung cấp option `zcago.WithDebugTrace(fn DebugTraceFn)` trong `options.go`.
- Struct `session.DebugTraceEvent` ghi nhận các trường chuẩn: `Kind` (`"http"`, `"api_response"`, `"ws_frame"`, `"ws_stage"`), `Phase`, `Endpoint`, `Method`, `Status`, `DurationMs`, `CMD`, `SubCMD`, `Bytes`, `Message`.
- `internal/httpx/client.go` tự động đo thời gian request và phát sự kiện trace sau mỗi lần `client.Do(req)`.

### 2.2. Hoàn Thiện Listener & WebSocket Controls
- Đồng bộ bắt gói tin cmd 551 như tin nhắn 1-1 thông thường (khớp commit `50b40b6` trong `zca-js`).
- Thêm sự kiện `ResetCloudEvent` (cmd 620) và `PhotoSuccessEvent` (cmd 601).
- Cơ chế drop-counter riêng biệt (`DroppedOptionalEvents`) cho các kênh tuỳ chọn, tránh làm sai lệch bộ đếm rớt tin chính (`DroppedEvents`).

---

## 3. Chi Tiết Phần B: Đột Phá REA Từ Luồng `debug/`

### 3.1. Cấu Trúc GID & Snowflake Epoch (Chứng Minh 344/344 Mẫu)
Zalo sử dụng cấu trúc Snowflake tùy biến để sinh định danh:
- **Snowflake Epoch:** `2011-01-10T17:41:26.666Z` (chính xác đến mili-giây).
- **Quy luật Bit:** 19 chữ số UID bao gồm $39\text{ bits timestamp} \ll 24 \mid 24\text{ bits random}$. Bit 63 luôn bằng 0 (dương) và Bit 15 ngẫu nhiên luôn bằng 0.
- **Cấu trúc GID:** 32 ký tự Base32Hex thuộc bảng chữ cái `0123456789ABCDEFGHIJKLMNOPQRSTUV`. Byte đệm cuối (pad-byte) luôn bằng 0 (`blob & 0xFF == 0`).
- **Các hàm trong `pkg/gid`:**
  - `UIDToTime(uid)`: Khôi phục chính xác thời điểm tạo tài khoản/entity.
  - `UIDBits(uid)`: Kiểm tra tính hợp lệ của bit63 và randBit15.
  - `IsWellFormedGID(gid)`: Kiểm tra cấu trúc GID hợp lệ.
  - `UIDToToolCol3(uid)` / `ToolCol3ToUID(col3)`: Giải mã thuật toán che dữ liệu nội bộ với khóa `0x20344cae784c20d6`.

### 3.2. Công Thức Chuyển Đổi UID Viewer Offline (0-Network)
Được chứng minh độc lập qua 34/34 mẫu holdout và kiểm tra chéo live giữa nhiều tài khoản:
$$\text{Mask}_{AB} = \text{send2me\_id}(A) \oplus \text{send2me\_id}(B)$$
$$\text{uid}_B = \text{uid}_A \oplus \text{Mask}_{AB}$$
- `send2me_id` là ID hội thoại "Cloud của tôi" lấy trực tiếp từ `loginInfo` khi đăng nhập (không thay đổi qua relogin).
- Cho phép 2 bot A và B trao đổi thông tin người dùng với nhau mà không cần gửi bất kỳ request nào lên server Zalo, độ trễ 0ms.
- Triển khai thread-safe trong `pkg/gid/store.go` với phương thức `GetOrComputeMask`.

### 3.3. Media URL Renewal (E-X3r) 2 Pha
Đặc tả bóc tách từ bundle Web `1.13ceb43a998bc76de243.js` và kiểm chứng live ngày 05/11/2026:
- **Pha 1 (HTTP POST):** Gửi tới endpoint `{fileBase}/api/message/renewlink` với `params` mã hóa AES-CBC.
  - Bắt buộc `isGroup = 1` nếu `toId` là nhóm hoặc `send2me_id` (Cloud của tôi).
  - Chuẩn hóa tự động các trường: `oriUrl`, `thumbUrl`, `hdUrl`, `normalUrl`, `width: 1`, `height: 1`, `isE2EE: 0`.
  - Phản hồi từ server: `{"error_code": 0, "data": {"async": true}}`.
- **Pha 2 (WebSocket Control Push):** Server Zalo đẩy kết quả qua WebSocket cmd 601:
  - `act_type: "photo"` | `act: "renew_link"`.
  - Trả về danh sách URL mới (`normal`, `hd`, `thumb`, `expiredTime`).
  - URL làm mới có hiệu lực dài hạn (ví dụ: `https://f16-zpcloud.zdn.vn/...` hạn tới tháng 02/2027, trả về HTTP/2 200).

### 3.4. Bản Chất Mã Lỗi QR -20
Phân tích kỹ thuật từ `libzproto_trust_ffi.so` và nhật ký kiểm thử:
- Mã `-20` sinh ra từ native Rust function `zproto_trust_decode_qr`.
- Xảy ra khi Zalo server yêu cầu xác thực thiết bị liên kết (`ntk` branch), nhưng client lại hiển thị mã QR theo định dạng classic thông thường.
- Đối với Web client (`apiType: 30`), việc duy trì `apiType: 30` cho phép vượt qua hoàn toàn nhánh `ntk` và đăng nhập QR thành công mà không gặp mã lỗi `-20`.

### 3.5. Cookie Jar Phân Vùng Đa Subdomain
- Chuẩn Go `net/http/cookiejar` tuân thủ nghiêm ngặt RFC 6265: nếu gọi `jar.SetCookies(https://chat.zalo.me, cookies)` nhưng trong danh sách có cookie domain `id.zalo.me` hoặc `zaloapp.com`, Go sẽ âm thầm loại bỏ các cookie này.
- Kết quả: Khi gọi sang `id.zalo.me`, các cookie thiết yếu như `zpdid`, `zlogin_session`, `zpw_sek` bị thiếu hụt dẫn đến lỗi phiên.
- **Giải pháp:** `BuildCookieJar` trong `credentials.go` tự động phân nhóm cookie theo đúng target domain trước khi nạp vào `jar`, bảo toàn 100% cookie trên mọi domain.

### 3.6. Kênh GID Member Thật (`GetMiniProfiles`) vs Bẫy Echo Của `group/members`
- **Bẫy Echo:** `/api/social/group/members` trả về trường `globalId` nhưng bị server echo chính GID của viewer đang gọi cho toàn bộ mọi thành viên (phát hiện và chứng minh trong thực nghiệm ngày 03/10/2026).
- **Kênh Thật:** Endpoint `/api/social/friend/getminiprofiles` (POST, mã hóa AES-CBC) là kênh chuẩn xác để lấy GID thật của thành viên:
  - Nhận tham số `{friend_ids: [...], avatar_size: 120, incInvalid: 1, srcReq: -1}`.
  - Phản hồi dạng map `{uid: {globalId, displayName, zaloName, avatar, bizPkg, ...}}`.
  - Tự động chunking tối đa 100 UIDs/request khớp hành vi client web Zalo.
  - Chuẩn hóa: `bizPkg` $\to$ `bizInfo`, `displayName` rỗng $\to$ fallback sang `zaloName`.

### 3.7. Khóa Tài Khoản Gốc UID-9 Trong Cookie Phiên
- Cookie `zpsid` và `zpw_sek` chứa định danh tài khoản 9 số cố định bất biến:
  - `zpsid` = `tag4.UID-9.gen.secret` (thời hạn 1 năm).
  - `zpw_sek` = `tag4.UID-9.a0.secret` (thời hạn 90 ngày).
- Đoạn thứ hai (`parts[1]`) chính là UID gốc tuần tự theo đăng ký (ví dụ: `243575631`).
- SDK tự động trích xuất `UID9` khi đăng nhập và cung cấp qua `client.GetUID9()`, `credentials.UID9()`, `session.ExtractUID9(val)`.

### 3.8. Triệt Tiêu Hardcode Test Fixtures Qua `internal/testenv`
- Toàn bộ các định danh test (`send2meA`, `send2meB`, `uidA`, `uid9`, `gidA`) được nạp động từ biến môi trường hoặc `.env.test`.
- Hàm `testenv.Load(t)` tự động kiểm định tính toàn vẹn: xác thực 64-bit unsigned integer, kiểm tra chiều dài GID 32 ký tự, và đối chiếu quan hệ toán học $\text{Send2meA} \oplus \text{Send2meB} = \text{ExpectedMask}$. Không một ID nào bị hardcode inline trong mã nguồn test.

### 3.9. Quy Trình Thu Hoạch Nhóm 2 Bước (`HarvestGroup`) & Linear AES-256-GCM
- **Thu Hoạch Nhóm 2 Bước:**
  1. Gọi `GetGroupInfo` (cmd `getmg-v2`) để lấy `GroupGlobalID` và danh sách toàn bộ thành viên `MemVerList`.
  2. Gọi `GetMiniProfiles` để lấy GID thật 100% của từng member (tránh bẫy echo-viewer).
  - Kết quả: Thu hoạch trọn gói nhóm trong đúng **2 HTTP request**, 0 request tới `/api/gid/*`, không lo bị rate limit.
- **Giải Mã Media E2EE (Linear AES-256-GCM):**
  - Cung cấp hàm `cryptox.DecodeAES256GCM(key, iv, tag, ct)` giải mã authenticated media với key 32B, IV 12B, Tag 16B.

### 3.10. Cỗ Máy Che Giấu Danh Tính ZNoise (Voicecall Signaling)
- **Bản Chất & Phạm Vi:**
  - Endpoint `GET /api/znoise` (cmd 12390) thuộc profile domain (`zwid.api.zalo.me`).
  - Trả về danh sách số âm 64-bit (`int64`, bit 63 = 1) tất định (deterministic) theo từng UID người dùng hoặc Group ID.
  - Sử dụng trong luồng báo hiệu cuộc gọi nhóm `getNoisedUids(partners, gids)` để che giấu UID thật giữa những người tham gia trước khi kết nối phòng thoại.
  - Được đóng gói qua phương thức `client.GetZNoise(ctx, req)`.

### 3.11. Bộ Mã Hóa Tham Số Socket Native (`encodeSocketParamsNative`)
- **Giải Mã Qua Ghidra & REA:**
  - Phân tích ngược hàm `CoreUtils.encodeSocketParamsNative` (VA `0x27ecc`) từ file native `libnative_utils.so`.
  - Cấu trúc 32-byte key: `[sequence 4B BE] || [privateKey 8B BE] || [const16 16B @0x18030] || [magic 4B LE 0x305b1f1d]`.
  - Hằng số `const16` trích xuất nguyên vẹn từ rodata: `48 ed 3a 1f 52 f9 39 41 e3 21 0d 3f 2e 5d e4 10`.
  - Thuật toán mã hóa: AES-256-CBC với vector khởi tạo zero (`IV = 0^16`) và padding PKCS#7, xuất ra chuỗi Base64 chuẩn.
  - Cung cấp trong `internal/cryptox.DeriveSocketKey`, `internal/cryptox.EncodeSocketParams`, và `internal/cryptox.DecodeSocketParams`.

---

## 4. Hướng Dẫn Sử Dụng Nhanh

### 4.1. Chuyển Đổi UID Giữa 2 Viewer
```go
import "github.com/luannt2002/zalo-api-unofficial/pkg/gid"

// Tính mask giữa Bot A và Bot B
mask, err := gid.MaskFromSend2me(send2meBotA, send2meBotB)

// Dịch UID người dùng từ góc nhìn Bot A sang Bot B
uidGocB, err := gid.TranslateViewerUID(uidGocA, send2meBotA, send2meBotB)
```

### 4.2. Làm Mới URL Media Hết Hạn (RenewLink)
```go
// 1. Lắng nghe kênh RenewLink từ listener
renewChan := client.Listener().RenewLink()

// 2. Gửi yêu cầu làm mới
res, err := client.RenewLink(ctx, api.RenewLinkRequest{
    ToID:    client.GetSend2meID(), // Hoặc ID nhóm
    MsgType: 2,                     // 2: Ảnh, 1: File, 5: Video
    MsgInfo: api.RenewMsgInfo{
        PhotoID:   "5900530976819228936",
        NormalURL: "https://f17-zpcloud.zdn.vn/.../image.jpg",
    },
})

// 3. Đón nhận URL mới đẩy về qua WebSocket
select {
case ev := <-renewChan:
    for _, item := range ev.Items {
        fmt.Println("URL mới:", item.Normal, "Hạn:", item.ExpiredTime)
    }
case <-time.After(10 * time.Second):
    fmt.Println("Hết thời gian chờ")
}
```

### 4.3. Lấy MiniProfile & GID Thật (Batching Tự Động)
```go
// Lấy profile kèm GID thật cho danh sách UID (tự chia batch ≤ 100/request)
profiles, err := client.GetMiniProfiles(ctx, []string{
    "2153857623592819130",
    "701985314262595886",
})
if err != nil {
    log.Fatal(err)
}

for uid, prof := range profiles {
    fmt.Printf("UID: %s | Tên: %s | GID Thật: %s\n", uid, prof.DisplayName, prof.GlobalID)
}
```

### 4.4. Lấy UID-9 Cố Định Xuyên Phiên
```go
// Trích xuất UID 9 số vĩnh viễn từ phiên đăng nhập
uid9 := client.GetUID9()
fmt.Println("UID gốc (9 chữ số):", uid9)
```

### 4.5. Thu Hoạch Nhóm Trọn Gói Trong 2 Request (`HarvestGroup`)
```go
// Thu hoạch toàn bộ metadata nhóm và GID thật của member (0 gọi /api/gid/*)
harvest, err := client.HarvestGroup(ctx, "8910587406831154166")
if err != nil {
    log.Fatal(err)
}

fmt.Println("Group GID:", harvest.GroupGlobalID)
fmt.Println("Tổng số member:", harvest.TotalMember)
for uid, gid := range harvest.Pairs {
    fmt.Printf("ID %s <-> GID %s\n", uid, gid)
}
```

### 4.6. Dịch GID Nhận Từ Bot Khác Thành UID Để Nhắn Tin (`ResolveUIDViaGID`)
```go
// Account B nhận GID từ Account A -> dịch sang UID góc nhìn của B
uids, err := client.ResolveUIDViaGID(ctx, "80NSGPJ9E4N56QVO6IEI7OOLJR7J21O0")
if err != nil {
    log.Fatal(err)
}

for gid, uid := range uids {
    fmt.Printf("GID: %s -> UID để gửi tin: %s\n", gid, uid)
}
```

### 4.7. Lấy ZNoise Cho Voicecall Nhóm (`GetZNoise`)
```go
// Lấy mã znoise (số âm i64) cho danh sách UID/GroupID trước khi báo hiệu voicecall
noise, err := client.GetZNoise(ctx, api.GetZNoiseRequest{
    FIDs: []string{"2153857623592819130"},
    GIDs: []string{"4558550755796016157"},
})
if err != nil {
    log.Fatal(err)
}

fmt.Printf("User Noised IDs: %v\n", noise.Data.FIDs)
fmt.Printf("Group Noised IDs: %v\n", noise.Data.GIDs)
```

### 4.8. Mã Hóa & Giải Mã Native Socket Parameters (`cryptox.EncodeSocketParams`)
```go
import "github.com/luannt2002/zalo-api-unofficial/internal/cryptox"

// Mã hóa tham số socket với sequence và server-issued private key
encoded, err := cryptox.EncodeSocketParams(1, 123456789012345678, "cmd=1380&type=call")
if err != nil {
    log.Fatal(err)
}

// Giải mã Base64 socket parameters
plaintext, err := cryptox.DecodeSocketParams(1, 123456789012345678, encoded)
```

### 4.9. Phân Loại Link Media & Tính Toán Thời Gian Hết Hạn (`pkg/media.Classify`)
```go
import "github.com/luannt2002/zalo-api-unofficial/pkg/media"

res := media.Classify("https://s240-26-ava-talk.zadn.vn/avatar.jpg?key=abcdefghijklmnopqrstuv&time=1795107600")
fmt.Println("Host Class:", res.HostClass)
if res.ExpiresAt != nil {
    fmt.Println("Hạn dùng:", res.ExpiresAt, "| Còn lại:", res.TimeRemaining)
}
```

### 4.10. Thanh Tra File Session Offline (`session.InspectSession`)
```go
import "github.com/luannt2002/zalo-api-unofficial/session"

data, _ := os.ReadFile("session.json")
diag, err := session.InspectSession(data)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("UID9: %s | Has Delete Markers: %v | Cookies: %d\n", diag.UID9, diag.HasDeleteMarkers, diag.CookieCount)
```

---

## 5. Trạng Thái Kiểm Thử (Verification Status)

- **Unit Tests với Race Detector:**
  ```bash
  go test -race -cover ./...
  # PASS tất cả các gói: pkg/gid (82.4%), listener (61.3%), errs (60.0%), internal (68.9%)
  ```
- **Static Analysis:**
  ```bash
  go vet ./...
  # 0 warnings, 0 errors
  ```
- **Live End-to-End Test:**
  Đã kiểm tra thành công với session thật `session-qr-live.json` thông qua script `e2e-renew-live.go`.

