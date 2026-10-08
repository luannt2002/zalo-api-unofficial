> Ghi chép lịch sử từ repo nguồn; không là bằng chứng bản tách này đã chạy tài khoản thật hoặc parity HEAD zca-js. Xem SOURCE-PROVENANCE.md và EXTRACTION-REPORT.md.

# Test Coverage & Luồng thực tế — zca-go (github.com/luannt2002/zalo-api-unofficial)

> Kết quả kiểm thử API với **Zalo THẬT** (bot Nguyễn Tường Luân), recipient test = 1 contact cố định.
> Cập nhật 2026-07-10. Xem thêm [API_REFERENCE.md](./API_REFERENCE.md) (từng func: service/endpoint/input/output).

## 0. Vì sao trước đây "dùng đến đâu lỗi đến đó"

Toàn bộ lịch sử dev/demo chạy **stub** (bot giả) + unit test với fake API → mọi test XANH nhưng chưa từng chạm Zalo thật. `github.com/luannt2002/zalo-api-unofficial` là **port của zca-js**; mỗi endpoint có bug port riêng chỉ lộ khi gọi Zalo THẬT. Vì vậy mỗi tính năng dùng lần đầu = 1 đường code chạm Zalo lần đầu = 1 bug tiềm ẩn. Cách chặn: **smoke-test từng endpoint với bot thật** (tài liệu này).

## 1. Ba KIỂU gọi Zalo (khác nhau — dễ nhầm)

| Kiểu | Cơ chế | Ví dụ | Yêu cầu input đặc thù |
|---|---|---|---|
| **Upload bytes** | Đẩy nội dung file lên Zalo | ảnh (`photo_original`), file (`asyncfile`) | ảnh cần `imageMetadataGetter` trả **Size** (⇒ `TotalChunk = ceil(Size/chunkSize)`; Size=0 ⇒ 0 chunk ⇒ photoId=-1). File chờ WS event `file_done`. |
| **URL fetch** | Đưa URL công khai, Zalo tự tải server-side | video, voice | Cần `MEDIA_PUBLIC_BASE` truy cập được từ ngoài. **Video BẮT BUỘC** `thumbUrl+duration+width+height` (thiếu ⇒ `ZaloAPIError[114]`). **Voice** cần m4a/aac + payload key `toId` (không phải `toid` ⇒ `111`). |
| **Params mã hóa** | POST `params` = AES-encrypt(payload JSON) | hầu hết action | `EncodeAES`; response `data` giải mã AES rồi unmarshal. |

## 2. Coverage (91 hàm SDK app dùng)

### ✅ Đã test THẬT — PASS (~50)
- **Gửi tin (10):** text, image, video, file, gif, sticker, voice, link, card, bankcard
- **Action tin/state (8):** react, forward, recall/undo, pin, mute, archive, unread-mark, typing
- **Contact mutation đảo-ngược (5):** alias set/remove, feed-block, block/unblock
- **Read (~24):** account info/settings/biz/avatars, friend recommendations/requests(incoming/received/sent), aliases, contact summary/last-online/related-group/find/by-phones, chats archived/pinned/hidden, sticker-categories, search-messages, quick-message-list, reminder-list, group info/members/link/invite-box, users-by-username

### ⚠️ Trả lỗi ĐÚNG (không phải bug SDK)
| Hàm | Kết quả | Lý do |
|---|---|---|
| GetCloseFriends, GetFriendOnlineStatus | Zalo **404 HTML** | Zalo đã BỎ endpoint (`/api/social/friend/getclosedfriends`, `/onlines`) → app map `FEATURE_DISABLED`, không retry |
| Catalog/Product* | `FEATURE_NOT_ENABLED` | Cần tài khoản **OA/doanh nghiệp**; bot cá nhân không có |
| Quick-message create | Zalo `821 Item size not support` | Ràng buộc input Zalo |
| Group history/blocked/pending | 403 RBAC / 400 param | Quyền / thiếu tham số |

### 🚫 CHƯA test THẬT (~35) — mutation tác động THẬT, không tự-dọn
Chỉ chạy khi có chủ đích rõ (tác động lên bạn bè/nhóm/tài khoản thật):
- **Kết bạn:** SendFriendRequest, Accept/Reject/Undo, RemoveFriend
- **Nhóm:** CreateGroup, Add/RemoveUser, ChangeOwner, Add/RemoveDeputy, Add/RemoveBlocked, Leave, Enable/DisableLink, JoinLink/InviteBox, ReviewPending, UpgradeCommunity, UpdateName/Avatar/Setting
- **Poll:** CreatePoll, Vote, Lock, Share, AddOptions
- **Reminder/Note:** Create/Update/Remove (Chatbotone nhận thông báo / thấy)
- **Hủy/nhạy cảm:** DeleteMessage, **SendReport** (tố cáo — có thể bị Zalo flag bot), UpdateProfile/Bio/Avatar/Language

## 3. Bug SDK/flow đã fix (11)

| # | Vùng | Bug | Fix |
|---|---|---|---|
| 1 | upload_attachment | `serviceURL += "/message"` mutate closure dùng chung ⇒ URL tích lũy `/message/message` (ảnh thứ 2+ 404) | biến local |
| 2 | send_message (image) | base attachment ký sớm (`MakeURL ...,true`) ⇒ path nối sau query ⇒ 404 | ký sau khi nối path (`false`) |
| 3 | proxy/imageMetadataGetter | thiếu `Size` ⇒ TotalChunk=0 ⇒ photoId=-1 | `os.Stat` trả Size |
| 4 | send_message/bank_card/link | `msgId` Zalo trả STRING hoặc NUMBER tùy endpoint | `flexID` UnmarshalJSON nhận cả hai |
| 5 | send_bank_card | `data` = string-trần HOẶC object `{msgId}` | UnmarshalJSON linh hoạt |
| 6 | send_voice | key `toid` ⇒ Zalo `111` | `toId` (camelCase) |
| 7 | send_video (app) | thiếu thumbUrl/duration/width/height ⇒ `114` | FE sinh poster (canvas) + đọc metadata |
| 8 | persist (app) | `genID=tsMs<<21\|hash21` 21-bit va chạm PK(id,ts) ⇒ batch kẹt (chặn cả lưu media inbound ⇒ "ảnh hết hạn") | `idgen` snowflake; idempotency do UNIQUE(bot,msg_id,ts) |
| 9 | outboundContent (app) | thiếu case "link" ⇒ preview rỗng | synth `{href}` |
| 10 | wrapZalo (app) | Zalo 404 ⇒ 503 retry-storm | +case 404 → FEATURE_DISABLED |
| 11 | threadPreview (FE) | nhãn `[Hình ảnh]` bị tưởng JSON ⇒ nuốt thành `[Tin nhắn]` | giữ nguyên nhãn `[chữ]` |

## 4. Cách smoke-test 1 endpoint (mẫu)

```bash
TOK=$(curl -s -X POST $API/v1/auth/login -d '{"phone":"...","password":"..."}' | jq -r .data.tokens.access_token)
# gửi: POST /v1/bots/{botID}/threads/{threadID}/messages   (msg_type: text|image|video|file|gif|sticker|voice|link|card|bankcard)
# đọc log kết quả:  docker logs zalo_all | grep "đã gửi\|ZaloAPIError"
# msg_id THẬT (không phải local:...) = tới Zalo OK; ZaloAPIError[N] = bug/ràng buộc endpoint N
```
