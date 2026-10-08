# Hướng dẫn sử dụng `github.com/luannt2002/zalo-api-unofficial`

> Tài liệu cách dùng **toàn bộ API** của thư viện `github.com/luannt2002/zalo-api-unofficial` — một **fork Go độc lập** của
> [zca-js](https://tdung.gitbook.io/zca-js) (bản gốc viết bằng TypeScript/Node.js).
>
> Mọi chữ ký hàm, tên struct request/response trong tài liệu này được **trích trực tiếp từ mã nguồn
> thật** của fork (`api/*.go`, `session/`, `listener/`, `model/`, `errs/`), KHÔNG sao chép từ bản gốc TS.

> ⚠️ **Cảnh báo:** đây là API không chính thức, hoạt động bằng cách giả lập Zalo Web. Việc dùng thư viện
> này có thể khiến tài khoản Zalo bị khóa/ban. Tự chịu rủi ro.

---

## Mục lục

1. [Giới thiệu & quy ước module](#1-giới-thiệu--quy-ước-module)
2. [Khởi tạo & đăng nhập](#2-khởi-tạo--đăng-nhập)
3. [Lắng nghe sự kiện (Listener)](#3-lắng-nghe-sự-kiện-listener)
4. [Gửi tin nhắn](#4-gửi-tin-nhắn)
5. [Thao tác trên tin nhắn](#5-thao-tác-trên-tin-nhắn)
6. [Bạn bè](#6-bạn-bè)
7. [Nhóm](#7-nhóm)
8. [Tài khoản & hồ sơ](#8-tài-khoản--hồ-sơ)
9. [Poll / Note / Reminder / Quick-message](#9-poll--note--reminder--quick-message)
10. [Sticker / Reaction / Catalog (zBusiness)](#10-sticker--reaction--catalog-zbusiness)
11. [Bảng tổng hợp toàn bộ API](#11-bảng-tổng-hợp-toàn-bộ-api)
12. [Mã lỗi & xử lý lỗi](#12-mã-lỗi--xử-lý-lỗi)
13. [Ghi chú khác biệt so với upstream](#13-ghi-chú-khác-biệt-so-với-upstream)
14. [Tham khảo](#14-tham-khảo)

---

## 1. Giới thiệu & quy ước module

- **Module name:** `github.com/luannt2002/zalo-api-unofficial` (xem `go.mod`). Đây là **fork riêng**, đã viết thêm/sửa nhiều API.
- **TUYỆT ĐỐI KHÔNG** import upstream `github.com/amrakk/zcago`. Import nội bộ theo package:

```go
import (
    zcago "github.com/luannt2002/zalo-api-unofficial"          // package gốc tên là `zcago` ở thư mục root module
    "github.com/luannt2002/zalo-api-unofficial/api"            // struct request/response + MessageContent, options...
    "github.com/luannt2002/zalo-api-unofficial/model"          // ThreadType, TMention, ReactionIcon, AttachmentSource, các event...
    "github.com/luannt2002/zalo-api-unofficial/listener"       // interface Listener
    "github.com/luannt2002/zalo-api-unofficial/session"        // session.Option, session.Context
    "github.com/luannt2002/zalo-api-unofficial/errs"           // ZaloAPIError, ZCAError, sentinel errors
)
```

> Lưu ý: package ở thư mục gốc module có **tên `package zcago`** (xem `zcago.go`). Khi import path là
> `github.com/luannt2002/zalo-api-unofficial`, nên đặt alias `zcago` cho rõ nghĩa như ví dụ trên.

Kiến trúc tổng quát:

- `zcago.NewZalo(opts...)` → đối tượng `Zalo` (chỉ có `Login` / `LoginQR`).
- Đăng nhập thành công → trả về `zcago.API` (interface lớn, gom **148 phương thức API** + 3 helper).
- `API.Listener()` → `listener.Listener` để nhận sự kiện realtime qua WebSocket.

---

## 2. Khởi tạo & đăng nhập

### 2.1 Tạo đối tượng Zalo + Options

```go
z := zcago.NewZalo(
    zcago.WithSelfListen(true),        // nhận cả tin do CHÍNH mình gửi trong listener
    zcago.WithCheckUpdate(false),      // tắt kiểm tra version upstream
    zcago.WithLogging(true),
    zcago.WithLogLevel(2),             // 0=Verbose 1=Debug 2=Info 3=Warn 4=Error 5=Success
    zcago.WithProxy("http://user:pass@host:port"), // egress IP riêng cho từng bot (chống ban IP)
    // zcago.WithHTTPClient(customClient),
    // zcago.WithImageMetadataGetter(fn), // BẮT BUỘC nếu gửi ảnh/file (lấy width/height/size)
)
```

Các option (file `options.go`, ánh xạ sang `session/options.go`):

| Option | Ý nghĩa |
|---|---|
| `WithSelfListen(bool)` | Listener có nhận tin do chính tài khoản gửi không |
| `WithCheckUpdate(bool)` | Bật/tắt kiểm tra cập nhật |
| `WithLogging(bool)` | Bật/tắt log |
| `WithLogLevel(uint8)` | Mức log 0..5 |
| `WithHTTPClient(*http.Client)` | HTTP client tùy biến |
| `WithProxy(string)` | Proxy HTTP/HTTPS cho cả REST + WebSocket (SOCKS5 chưa hỗ trợ) |
| `WithImageMetadataGetter(fn)` | Hàm lấy metadata ảnh — bắt buộc khi gửi ảnh/gif/file |
| `WithAPIType(uint)` / `WithAPIVersion(uint)` | Tham số API type/version |

### 2.2 Đăng nhập bằng QR (`session/auth/login_qr.go`)

```go
ctx := context.Background()

a, err := z.LoginQR(ctx, &zcago.LoginQROption{
    UserAgent: "Mozilla/5.0 ...",   // tùy chọn; mặc định config.DefaultUserAgent
    Language:  "vi",                // tùy chọn; mặc định config.DefaultLanguage
    QRPath:    "./qr.png",          // tùy chọn: lưu ảnh QR ra file
}, func(ev auth.LoginQREvent) {     // callback nhận các pha của QR; có thể truyền nil
    switch e := ev.(type) {
    case auth.EventQRCodeGenerated:
        // e.Data.Image là ảnh QR base64; e.Data.Code là mã
        // e.Actions.SaveToFile(ctx, "qr.png"), e.Actions.Retry(ctx), e.Actions.Abort(ctx)
    case auth.EventQRCodeScanned:
        // e.Data.DisplayName, e.Data.Avatar
    case auth.EventQRCodeDeclined:
        // người dùng từ chối
    case auth.EventQRCodeExpired:
        // QR hết hạn -> e.Actions.Retry(ctx)
    case auth.EventGotLoginInfo:
        // e.Data.IMEI, e.Data.UserAgent — LƯU LẠI để login cookie lần sau
    }
})
if err != nil {
    log.Fatal(err)
}
```

`LoginQROption` (file `options.go`): `UserAgent`, `Language`, `QRPath`.
Các sự kiện QR và payload nằm ở `session/auth/types.go` (import `github.com/luannt2002/zalo-api-unofficial/session/auth`).

### 2.3 Đăng nhập bằng cookie đã lưu (`Login`)

`Credentials` (file `credentials.go`):

```go
type Credentials struct {
    IMEI      string       `json:"imei"`
    Cookie    *CookieUnion `json:"cookie"`
    UserAgent string       `json:"userAgent"`
    Language  *string      `json:"language,omitempty"`
}
```

`CookieUnion` chấp nhận **2 định dạng**:
- Mảng cookie: `zcago.NewCookieArray([]zcago.Cookie{...})`
- Đối tượng J2: `zcago.NewJ2Cookie(zcago.J2Cookie{URL: "...", Cookies: []zcago.Cookie{...}})`
- Hoặc từ `[]*http.Cookie`: `zcago.NewHTTPCookie(httpCookies)`

```go
lang := "vi"
cookie := zcago.NewCookieArray([]zcago.Cookie{
    {Name: "...", Value: "...", Domain: ".zalo.me", Path: "/"}, // KHÔNG ghi cookie thật vào tài liệu/commit
})
cred := zcago.NewCredentials("imei-da-luu", cookie, "user-agent-da-luu", &lang)

a, err := z.Login(ctx, cred)
if err != nil {
    log.Fatal(err)
}
```

> `Credentials.IsValid()` yêu cầu `IMEI` và `UserAgent` không rỗng; `Cookie` có thể nil (khi đó dùng
> cookie sẵn trong session, ví dụ ngay sau `LoginQR`).

### 2.4 Lấy API object & helper

```go
ownID := a.GetOwnID()              // ID tài khoản đang đăng nhập (string)
sc, err := a.GetContext()          // session.Context
ln := a.Listener()                 // listener.Listener
```

---

## 3. Lắng nghe sự kiện (Listener)

Interface `listener.Listener` (file `listener/listener.go`) phơi bày các **kênh (channel)** Go, mỗi loại
sự kiện một channel. Bạn `Start` listener rồi `select` trên các channel.

```go
ln := a.Listener()
// retryOnClose=true: tự kết nối lại theo cấu hình retry của Zalo
if err := ln.Start(ctx, true); err != nil {
    log.Fatal(err)
}
defer ln.Stop()

for {
    select {
    case <-ctx.Done():
        return
    case m, ok := <-ln.Message():
        if !ok { return }
        switch v := m.(type) {
        case model.UserMessage:   // tin nhắn 1-1 (DM)
            _ = v
        case model.GroupMessage:  // tin nhắn nhóm
            _ = v
        }
    case u := <-ln.Undo():            // có người thu hồi tin (model.Undo)
        _ = u
    case r := <-ln.Reaction():        // có reaction mới (model.Reaction)
        _ = r
    case fe := <-ln.Friend():         // sự kiện bạn bè (model.FriendEvent)
        _ = fe
    case ge := <-ln.Group():          // sự kiện nhóm (model.GroupEvent)
        _ = ge
    case t := <-ln.Typing():          // đang gõ (model.Typing)
        _ = t
    case ci := <-ln.Closed():         // kết nối đóng hẳn (websocketx.CloseInfo)
        _ = ci
        return
    case err := <-ln.Error():
        log.Println("listener error:", err)
    }
}
```

Các kênh đầy đủ trên `Listener`:

| Method channel | Kiểu dữ liệu | Ý nghĩa |
|---|---|---|
| `Connected()` | `<-chan struct{}` | Đã kết nối WebSocket |
| `Disconnected()` | `<-chan CloseInfo` | Mất kết nối (có thể tự retry) |
| `Closed()` | `<-chan CloseInfo` | Đóng hẳn, không retry nữa |
| `Error()` | `<-chan error` | Lỗi runtime |
| `Message()` | `<-chan model.Message` | Tin mới (`UserMessage` / `GroupMessage`) |
| `OldMessages()` | `<-chan model.OldMessages` | Tin cũ (sau `RequestOldMessages`) |
| `Reaction()` | `<-chan model.Reaction` | Reaction mới |
| `OldReactions()` | `<-chan model.OldReactions` | Reaction cũ |
| `Typing()` | `<-chan model.Typing` | Sự kiện đang gõ |
| `DeliveredMessages()` | `<-chan []model.DeliveredMessage` | Tin đã được nhận (có thể trùng) |
| `SeenMessages()` | `<-chan []model.SeenMessage` | Tin đã xem |
| `Undo()` | `<-chan model.Undo` | Thu hồi tin |
| `UploadAttachment()` | `<-chan model.UploadAttachment` | Kết quả upload đính kèm |
| `Friend()` | `<-chan model.FriendEvent` | Sự kiện bạn bè |
| `Group()` | `<-chan model.GroupEvent` | Sự kiện nhóm |
| `CipherKey()` | `<-chan string` | Khóa mã hóa |

Hàm bổ sung:

```go
ln.RequestOldMessages(ctx, model.ThreadTypeUser, nil)   // yêu cầu tải tin cũ
ln.RequestOldReactions(ctx, model.ThreadTypeGroup, nil)
ln.SendWS(ctx, payload, true)                            // gửi gói WebSocket thô
```

**Loại sự kiện** (giá trị enum trong `model/friend_event.go`, `model/group_event.go`):

- `FriendEvent`: `add`, `remove`, `req_v2` (gửi lời mời), `undo_req`, `reject`, `seen_fr_req`,
  `block`, `unblock`, `block_call`, ...
- `GroupEvent`: `join_request`, `join`, `leave`, `remove_member`, `block_member`, `update_setting`,
  `update`, `new_link`, `add_admin`, `remove_admin`, `new_pin_topic`, `update_board`, `update_topic`, ...

> ⚠️ **Mỗi tài khoản chỉ chạy được 1 listener Web tại một thời điểm.** Nếu mở Zalo trên trình duyệt
> khi listener đang chạy, listener sẽ bị Zalo ngắt (close code `3000`/`3003`).

---

## 4. Gửi tin nhắn

Tất cả hàm gửi nhận `threadID string` và `threadType model.ThreadType`
(`model.ThreadTypeUser` cho DM, `model.ThreadTypeGroup` cho nhóm).

### 4.1 Text, style, mention, reply/quote, TTL — `SendMessage`

`MessageContent` (file `api/send_message.go`):

```go
type MessageContent struct {
    Msg         string
    Style       []MessageStyle           // in đậm/nghiêng/màu...
    Urgency     model.Urgency            // UrgDefault / UrgImportant / UrgUrgent
    Quote       *SendMessageQuote        // trả lời / trích dẫn
    Mentions    []model.TMention         // tag thành viên (chỉ nhóm)
    Attachments []model.AttachmentSource // gửi kèm ảnh/file
    TTL         int                      // mili-giây, tin tự xóa
}
```

```go
// Text thường
resp, err := a.SendMessage(ctx, userID, model.ThreadTypeUser,
    api.MessageContent{Msg: "Xin chào!"})
log.Println(resp.Message.MsgID)

// In đậm phần "đậm"
a.SendMessage(ctx, userID, model.ThreadTypeUser, api.MessageContent{
    Msg:   "chữ đậm và thường",
    Style: []api.MessageStyle{{Start: 0, Len: 7, Style: api.TextStyleBold}},
})

// Mention trong NHÓM (UID "-1" = mention all)
a.SendMessage(ctx, groupID, model.ThreadTypeGroup, api.MessageContent{
    Msg:      "@An ơi check nhé",
    Mentions: []model.TMention{{UID: "1234", Pos: 0, Len: 3, Type: model.MentionEach}},
})

// Reply / quote một tin
a.SendMessage(ctx, groupID, model.ThreadTypeGroup, api.MessageContent{
    Msg: "trả lời nè",
    Quote: &api.SendMessageQuote{
        MsgID: "...", CliMsgID: "...", MsgType: "webchat", UIDFrom: "...",
        TS: "...", TTL: 0,
    },
})
```

`MessageStyle.Style` dùng hằng `api.TextStyle*`: `Bold`, `Italic`, `Underline`, `StrikeThrough`,
`Red/Orange/Yellow/Green`, `Small/Big`, `OrderedList/UnorderedList`, `Indent`.

> Gửi ảnh/file kèm text: đưa file vào `Attachments` (xem 4.7). Cần `WithImageMetadataGetter` để gửi ảnh.

### 4.2 Sticker — `SendSticker`

```go
a.SendSticker(ctx, userID, model.ThreadTypeUser, api.SendStickerPayload{
    ID: 12345, CateID: 10, Type: 7,
})
```

### 4.3 GIF — `SendGIF`

```go
src := model.NewStringAttachment("./funny.gif")
a.SendGIF(ctx, userID, model.ThreadTypeUser, api.GIFContent{
    Attachment: src,
    TTL:        0,
})
```

### 4.4 Video — `SendVideo` (gửi bằng URL)

```go
a.SendVideo(ctx, userID, model.ThreadTypeUser, api.SendVideoOptions{
    VideoURL:     "https://.../clip.mp4",
    ThumbnailURL: "https://.../thumb.jpg",
    Duration:     12000, Width: 1280, Height: 720,
    Msg:          "video nè",
})
```

### 4.5 Voice — `SendVoice` (gửi bằng URL)

```go
a.SendVoice(ctx, userID, model.ThreadTypeUser, api.SendVoiceOptions{
    VoiceURL: "https://.../voice.aac", TTL: 0,
})
```

### 4.6 Danh thiếp (contact card) — `SendCard`

```go
a.SendCard(ctx, userID, model.ThreadTypeUser, api.SendCardOptions{
    UserID:      "987654321",   // chia sẻ danh thiếp của user này
    PhoneNumber: "84901234567", // hoặc theo số điện thoại
})
```

Thẻ ngân hàng — `SendBankCard` (dùng `api.SendBankCardData`, BIN bank xem `model.BinBankCard`).

### 4.7 File / ảnh đính kèm — `UploadAttachment` + `SendMessage`

`model.AttachmentSource` tạo từ:
- Đường dẫn file: `model.NewStringAttachment("./a.png")`
- Đối tượng đọc trực tiếp: `model.NewObjectAttachment(filename, model.AttachmentMetadata{...}, reader)`

```go
// Gửi ảnh kèm caption qua SendMessage
a.SendMessage(ctx, userID, model.ThreadTypeUser, api.MessageContent{
    Msg:         "ảnh đây",
    Attachments: []model.AttachmentSource{model.NewStringAttachment("./photo.jpg")},
})

// Hoặc upload trước rồi xử lý thủ công
up, err := a.UploadAttachment(ctx, userID, model.ThreadTypeUser,
    model.NewStringAttachment("./doc.pdf"))
```

### 4.8 Link — `SendLink`

```go
a.SendLink(ctx, userID, model.ThreadTypeUser, api.SendLinkOptions{
    Msg:  "xem link này",
    Link: "https://example.com",
})
```

### 4.9 Sự kiện gõ / đã nhận / đã xem

```go
a.SendTypingEvent(ctx, userID, model.ThreadTypeUser, model.DestTypeUser)
a.SendSeenEvent(ctx, []model.OutboundMessage{...}, model.ThreadTypeUser)
a.SendDeliveredEvent(ctx, []model.OutboundMessage{...}, model.ThreadTypeUser, true)
```

---

## 5. Thao tác trên tin nhắn

### 5.1 Thu hồi (recall/undo) — `UndoMessage`

Chỉ thu hồi tin **do chính mình gửi**, trong vòng 24h.

```go
a.UndoMessage(ctx, userID, model.ThreadTypeUser, api.UndoMessageData{
    MsgID: "...", CliMsgID: "...",
})
```

### 5.2 Xóa tin — `DeleteMessage`

```go
a.DeleteMessage(ctx, api.DeleteMessageDestination{
    ThreadID: groupID,
    Type:     model.ThreadTypeGroup,
    Data:     api.DeleteMessageData{MsgID: "...", CliMsgID: "...", UIDFrom: "..."},
}, true) // onlyMe=true: chỉ ẩn với mình
```
> Xóa cho mọi người trong DM không hỗ trợ — dùng `UndoMessage`. Lỗi liên quan:
> `api.ErrSenderRecall`, `api.ErrDMRecipientsDeleteUnsupported`.

### 5.3 React — `AddReaction`

```go
a.AddReaction(ctx, api.AddReactionDestination{
    ThreadID: userID,
    Type:     model.ThreadTypeUser,
    Data:     api.AddReactionData{MsgID: "...", CliMsgID: "..."},
}, model.NewReactionData(model.ReactionHeart))
```
Danh sách icon: `model.ReactionHeart`, `ReactionLike`, `ReactionHaha`, `ReactionWow`, `ReactionCry`,
`ReactionAngry`, `ReactionKiss`, ... (đầy đủ trong `model/reaction.go`). Bỏ reaction: `ReactionNone`.

### 5.4 Chuyển tiếp — `ForwardMessage`

```go
a.ForwardMessage(ctx, []string{idA, idB}, model.ThreadTypeUser, api.ForwardMessagePayload{
    Message: "nội dung forward",
})
```

### 5.5 Ghim hội thoại — `SetPinChat`

```go
a.SetPinChat(ctx, []string{userID}, model.ThreadTypeUser, true)
```

### 5.6 Đánh dấu đã/chưa đọc

```go
a.AddUnreadMark(ctx, userID, model.ThreadTypeUser)    // đánh dấu chưa đọc
a.RemoveUnreadMark(ctx, userID, model.ThreadTypeUser) // bỏ dấu chưa đọc
a.GetUnreadMark(ctx)
```

Khác: `SetMute`, `GetMute`, `SetHiddenChat`, `DeleteChat`, `UpdateAutoDeleteChat`,
`GetArchivedChatList` / `UpdateArchivedChatList`.

---

## 6. Bạn bè

### 6.1 Tìm theo số điện thoại — `FindUser`

> **Định dạng số:** thêm mã quốc gia, ví dụ Việt Nam `84xxxxxxxxx` (KHÔNG dùng số 0 đầu).

```go
res, err := a.FindUser(ctx, "84901234567")
// res là map[phone]struct{ UID, DisplayName, ZaloName, Avatar, Gender, ... }
```
Nhiều số: `a.FindUser(ctx, "8490...", "8491...")`. Lỗi: `api.ErrPhoneNumberEmpty`.
Tìm theo username: `FindUserByUsername`. Nhiều số một lần: `GetMultiUsersByPhones`.

### 6.2 Thông tin người dùng

```go
a.GetUserInfo(ctx, userID)          // 1 hoặc nhiều userID
a.GetUserSummaryInfo(ctx, userID)
a.GetAllFriends(ctx, model.OffsetPaginationOptions{Count: 20000, Page: 1})
a.GetCloseFriends(ctx)
a.GetFriendOnlineStatus(ctx)
a.GetFriendRecommendations(ctx)
a.LastOnline(ctx, userID)
```

### 6.3 Lời mời kết bạn

```go
a.SendFriendRequest(ctx, "Chào bạn, kết bạn nhé", userID)
a.AcceptFriendRequest(ctx, friendID)
a.RejectFriendRequest(ctx, friendID)
a.UndoFriendRequest(ctx, userID)            // hủy lời mời đã gửi
a.GetSentFriendRequest(ctx)
a.GetFriendRequestStatus(ctx, friendID)
```

### 6.4 Xóa bạn / chặn

```go
a.RemoveFriend(ctx, friendID)
a.BlockUser(ctx, userID)
a.UnblockUser(ctx, userID)
a.SetViewFeedBlock(ctx, userID, true)
```

### 6.5 Tên gợi nhớ (alias)

```go
a.UpdateAlias(ctx, userID, "Sếp An")
a.RemoveAlias(ctx, userID)
a.GetAliasList(ctx, model.OffsetPaginationOptions{Count: 100, Page: 1})
```

---

## 7. Nhóm

### 7.1 Tạo nhóm — `CreateGroup`

```go
g, err := a.CreateGroup(ctx, api.CreateGroupOptions{
    Name:    "Nhóm dự án",
    Members: []string{"123", "456"}, // bắt buộc >= 1 thành viên
})
log.Println(g.GroupID, g.SuccessMembers, g.ErrorMembers)
```

### 7.2 Thành viên

```go
a.AddUserToGroup(ctx, groupID, "123", "456")
a.RemoveUserFromGroup(ctx, groupID, "123")
a.GetGroupMembersInfo(ctx, memberID...)
a.InviteUserToGroups(ctx, userID, groupID1, groupID2)
a.LeaveGroup(ctx, groupID, false)
a.DeleteGroup(ctx, groupID)                 // giải tán nhóm
```

### 7.3 Tên / avatar / cài đặt

```go
a.UpdateGroupName(ctx, groupID, "Tên mới")
a.UpdateGroupAvatar(ctx, groupID, model.NewStringAttachment("./avatar.png"))
a.UpdateGroupSetting(ctx, groupID, api.UpdateGroupSettingOptions{ /* ... */ })
a.GetGroupInfo(ctx, groupID)                // 1 hoặc nhiều
a.GetAllGroups(ctx)
a.GetGroupChatHistory(ctx, groupID, 50)
a.UpgradeGroupToCommunity(ctx, groupID)
```

### 7.4 Trưởng/phó nhóm (owner/deputy)

```go
a.ChangeGroupOwner(ctx, groupID, memberID)  // chuyển chủ nhóm (mất quyền admin hiện tại)
a.AddGroupDeputy(ctx, groupID, "123")
a.RemoveGroupDeputy(ctx, groupID, "123")
```

### 7.5 Link nhóm

```go
a.EnableGroupLink(ctx, groupID)
a.DisableGroupLink(ctx, groupID)
a.GetGroupLinkDetail(ctx, groupID)
a.GetGroupLinkInfo(ctx, "https://zalo.me/g/...", 1)
a.JoinGroupLink(ctx, "https://zalo.me/g/...")
```

### 7.6 Duyệt thành viên & chặn trong nhóm (pending)

```go
a.GetGroupPendingJoinRequests(ctx, groupID)
a.ReviewPendingMemberRequest(ctx, groupID, api.ReviewPendingMemberData{ /* ... */ })
a.AddGroupBlockedMember(ctx, groupID, "123")
a.RemoveGroupBlockedMember(ctx, groupID, "123")
a.GetGroupBlockedMember(ctx, groupID, model.OffsetPaginationOptions{Count: 50, Page: 1})
```

Invite box: `GetGroupInviteBoxList`, `GetGroupInviteBoxInfo`, `JoinGroupInviteBox`, `RemoveGroupInviteBox`.

---

## 8. Tài khoản & hồ sơ

```go
a.GetAccountInfo(ctx)
a.GetSetting(ctx)
a.UpdateSetting(ctx, api.UpdateSettingType(/*...*/), 1)
a.UpdateProfile(ctx, api.UpdateProfileData{ /* ... */ })
a.UpdateProfileBio(ctx, "Trạng thái mới")
a.UpdateAccountAvatar(ctx, model.NewStringAttachment("./me.png"))
a.UpdateActiveStatus(ctx, true)
a.UpdateLanguage(ctx, api.Language("VI"))   // "VI" | "EN"
a.GetAvatarList(ctx, model.OffsetPaginationOptions{Count: 50, Page: 1})
a.ReuseAvatar(ctx, photoID)
a.DeleteAvatar(ctx, photoID)
a.GetQR(ctx, userID)                        // QR danh thiếp
a.GetCookie(ctx)                            // lấy cookie hiện tại (để lưu cho login lần sau)
a.KeepAlive(ctx)
```

zBusiness: `GetBizAccount`, `GetAutoReplyList`, `CreateAutoReply`, `UpdateAutoReply`, `DeleteAutoReply`.

---

## 9. Poll / Note / Reminder / Quick-message

### 9.1 Poll (bình chọn trong nhóm)

```go
p, _ := a.CreatePoll(ctx, groupID, api.CreatePollOptions{
    Question:          "Ăn gì trưa nay?",
    Options:           []string{"Cơm", "Phở", "Bún"},
    AllowMultiChoices: true,
    AllowAddNewOption: false,
    IsAnonymous:       false,
    // ExpiredTime, HideVotePreview...
})
a.VotePoll(ctx, "pollID", 0, 1)        // optionID là index/ID
a.AddPollOptions(ctx, api.AddPollOptionsRequest{ /* ... */ })
a.GetPollDetail(ctx, pollIDInt)
a.LockPoll(ctx, pollIDInt)
a.SharePoll(ctx, pollIDInt)
```

### 9.2 Note (ghi chú nhóm)

```go
a.CreateNote(ctx, groupID, api.CreateNoteOptions{Title: "Ghi chú", PinAct: true})
a.UpdateNote(ctx, groupID, api.UpdateNoteOptions{ /* ... */ })
```

### 9.3 Reminder (nhắc hẹn)

```go
a.CreateReminder(ctx, threadID, model.ThreadTypeGroup, api.CreateReminderOptions{ /* ... */ })
a.UpdateReminder(ctx, threadID, model.ThreadTypeGroup, api.UpdateReminderOptions{ /* ... */ })
a.RemoveReminder(ctx, threadID, model.ThreadTypeGroup, reminderID)
a.GetReminder(ctx, reminderID)
a.GetReminderList(ctx, threadID, model.ThreadTypeGroup, model.OffsetPaginationOptions{Count: 20, Page: 1})
a.GetReminderResponse(ctx, reminderID)
```

### 9.4 Quick message (tin nhắn nhanh)

```go
a.AddQuickMessage(ctx, api.AddQuickMessageRequest{ /* ... */ })
a.UpdateQuickMessage(ctx, itemID, api.UpdateQuickMessageData{ /* ... */ })
a.RemoveQuickMessage(ctx, msgID)
a.GetQuickMessageList(ctx)
```

---

## 10. Sticker / Reaction / Catalog (zBusiness)

### 10.1 Sticker

```go
a.GetStickers(ctx, "love")                 // trả về danh sách sticker ID
a.GetStickerDetail(ctx, stickerID)
a.GetStickersDetail(ctx, id1, id2)
a.SearchSticker(ctx, "tet", 20)
a.GetStickerCategoryDetail(ctx, cateID)
```

### 10.2 Reaction list

Danh sách icon hợp lệ là hằng `model.Reaction*` trong `model/reaction.go`; tạo payload bằng
`model.NewReactionData(icon)` (xem mục 5.3).

### 10.3 Catalog & sản phẩm (zBusiness)

```go
a.GetCatalogList(ctx, api.GetCatalogListRequest{ /* ... */ })
a.CreateCatalog(ctx, "Tên catalog")
a.UpdateCatalog(ctx, api.UpdateCatalogRequest{ /* ... */ })
a.DeleteCatalog(ctx, catalogID)
a.CreateProductCatalog(ctx, api.CreateProductCatalogRequest{ /* ... */ })
a.GetProductCatalogList(ctx, api.GetProductCatalogListRequest{ /* ... */ })
a.UpdateProductCatalog(ctx, api.UpdateProductCatalogRequest{ /* ... */ })
a.DeleteProductCatalog(ctx, api.DeleteProductCatalogRequest{ /* ... */ })
a.UploadProductPhoto(ctx, model.NewStringAttachment("./p.jpg"))
```

---

## 11. Bảng tổng hợp toàn bộ API

> Tổng cộng **148 phương thức API** trên interface `zcago.API` (chưa kể 3 helper `GetContext`,
> `GetOwnID`, `Listener`). Cột "Req" / "Resp" ghi struct chính; file ở thư mục `api/`.
> `TT` = `model.ThreadType`. Kiểu trả về `*X` nghĩa là con trỏ.

### 11.1 Gửi tin & thao tác tin

| Hàm Go | Mô tả | Req chính | Resp | File |
|---|---|---|---|---|
| `SendMessage` | Gửi tin (text/style/mention/quote/đính kèm) | `MessageContent` | `*SendMessageResponse` | send_message.go |
| `SendSticker` | Gửi sticker | `SendStickerPayload` | `*SendStickerResponse` | send_sticker.go |
| `SendGIF` | Gửi GIF | `GIFContent` | `*SendGIFResponse` | send_gif.go |
| `SendVideo` | Gửi video (URL) | `SendVideoOptions` | `*SendVideoResponse` | send_video.go |
| `SendVoice` | Gửi voice (URL) | `SendVoiceOptions` | `*SendVoiceResponse` | send_voice.go |
| `SendLink` | Gửi link | `SendLinkOptions` | `*SendLinkResponse` | send_link.go |
| `SendCard` | Gửi danh thiếp | `SendCardOptions` | `*SendCardResponse` | send_card.go |
| `SendBankCard` | Gửi thẻ ngân hàng | `SendBankCardData` | `SendBankCardResponse` | send_bank_card.go |
| `ForwardMessage` | Chuyển tiếp tin | `ForwardMessagePayload` | `*ForwardMessageResponse` | forward_message.go |
| `UndoMessage` | Thu hồi tin của mình | `UndoMessageData` | `*UndoMessageResponse` | undo_message.go |
| `DeleteMessage` | Xóa tin | `DeleteMessageDestination`,`bool` | `*DeleteMessageResponse` | delete_message.go |
| `AddReaction` | React tin | `AddReactionDestination`,`model.ReactionData` | `*AddReactionResponse` | add_reaction.go |
| `SendTypingEvent` | Sự kiện đang gõ | `threadID,TT,model.DestType` | `*SendTypingEventResponse` | send_typing_event.go |
| `SendSeenEvent` | Sự kiện đã xem | `[]model.OutboundMessage,TT` | `*SendSeenEventResponse` | send_seen_event.go |
| `SendDeliveredEvent` | Sự kiện đã nhận | `[]model.OutboundMessage,TT,bool` | `SendDeliveredEventResponse` | send_delivered_event.go |
| `SendReport` | Báo cáo hội thoại | `SendReportOptions` | `*SendReportResponse` | send_report.go |
| `UploadAttachment` | Upload đính kèm | `...model.AttachmentSource` | `UploadAttachmentResponse` | upload_attachment.go |
| `UploadPhoto` | Upload ảnh | `model.AttachmentSource` | `*UploadPhotoResponse` | upload_photo.go |
| `UploadThumbnail` | Upload thumbnail | `model.AttachmentSource` | `*UploadThumbnailResponse` | upload_thumbnail.go |
| `UploadProductPhoto` | Upload ảnh sản phẩm | `model.AttachmentSource` | `*UploadProductPhotoResponse` | upload_product_photo.go |
| `ParseLink` | Phân tích metadata link | `link string` | `*ParseLinkResponse` | parse_link.go |

### 11.2 Hội thoại (pin/mute/hidden/archive/unread)

| Hàm Go | Mô tả | Req chính | Resp | File |
|---|---|---|---|---|
| `SetPinChat` | Ghim/bỏ ghim | `[]string,TT,bool` | `SetPinChatResponse` | set_pin_chat.go |
| `GetPinnedChat` | DS đã ghim | — | `*GetPinnedChatResponse` | get_pinned_chat.go |
| `SetMute` | Tắt thông báo | `threadID,TT,SetMuteOptions` | `SetMuteResponse` | set_mute.go |
| `GetMute` | DS đang tắt tiếng | — | `*GetMuteResponse` | get_mute.go |
| `SetHiddenChat` | Ẩn/hiện hội thoại | `[]string,TT,bool` | `SetHiddenChatResponse` | set_hidden_chat.go |
| `GetHiddenChat` | DS hội thoại ẩn | — | `*GetHiddenChatResponse` | get_hidden_chat.go |
| `UpdateHiddenChatPIN` | Đổi PIN hội thoại ẩn | `pin string` | `UpdateHiddenChatPINResponse` | update_hidden_chat_pin.go |
| `ResetHiddenChatPIN` | Reset PIN hội thoại ẩn | — | `ResetHiddenChatPINResponse` | reset_hidden_chat_pin.go |
| `DeleteChat` | Xóa hội thoại | `threadID,TT,DeleteChatLastMessage` | `*DeleteChatResponse` | delete_chat.go |
| `GetArchivedChatList` | DS lưu trữ | — | `*GetArchivedChatListResponse` | get_archived_chat_list.go |
| `UpdateArchivedChatList` | Cập nhật lưu trữ | `bool,...UpdateArchivedChatListTarget` | `*UpdateArchivedChatListResponse` | update_archived_chat_list.go |
| `GetAutoDeleteChat` | DS tự xóa | — | `*GetAutoDeleteChatResponse` | get_auto_delete_chat.go |
| `UpdateAutoDeleteChat` | Bật tự xóa | `threadID,TT,ChatTTL` | `UpdateAutoDeleteChatResponse` | update_auto_delete_chat.go |
| `AddUnreadMark` | Đánh dấu chưa đọc | `threadID,TT` | `*AddUnreadMarkResponse` | add_unread_mark.go |
| `RemoveUnreadMark` | Bỏ dấu chưa đọc | `threadID,TT` | `*RemoveUnreadMarkResponse` | remove_unread_mark.go |
| `GetUnreadMark` | Trạng thái chưa đọc | — | `*GetUnreadMarkResponse` | get_unread_mark.go |
| `GetLabels` | Lấy nhãn | — | `*GetLabelsResponse` | get_labels.go |
| `UpdateLabels` | Cập nhật nhãn | `UpdateLabelsData` | `*UpdateLabelsResponse` | update_labels.go |

### 11.3 Bạn bè

| Hàm Go | Mô tả | Req chính | Resp | File |
|---|---|---|---|---|
| `FindUser` | Tìm theo SĐT (84...) | `...string` | `*FindUserResponse` | find_user.go |
| `FindUserByUsername` | Tìm theo username | `username,...AvatarSize` | `*FindUserByUsernameResponse` | find_user_by_username.go |
| `GetMultiUsersByPhones` | Nhiều user theo SĐT | `[]string,...AvatarSize` | `*GetMultiUsersByPhonesResponse` | get_multi_users_by_phones.go |
| `GetUserInfo` | Thông tin user | `...string` | `*GetUserInfoResponse` | get_user_info.go |
| `GetUserSummaryInfo` | Thông tin tóm tắt | `...string` | `*GetUserSummaryInfoResponse` | get_user_summary_info.go |
| `GetAllFriends` | Tất cả bạn bè | `OffsetPaginationOptions` | `*GetAllFriendsResponse` | get_all_friends.go |
| `GetCloseFriends` | Bạn thân | — | `*GetCloseFriendsResponse` | get_close_friends.go |
| `GetFriendOnlineStatus` | Bạn đang online | — | `*GetFriendOnlineStatusResponse` | get_friend_online_status.go |
| `GetFriendRecommendations` | Gợi ý kết bạn | — | `*GetFriendRecommendationsResponse` | get_friend_recommendations.go |
| `GetRelatedFriendGroup` | Nhóm bạn chung | `...string` | `*GetRelatedFriendGroupResponse` | get_related_friend_group.go |
| `LastOnline` | Lần online cuối | `userID` | `*LastOnlineResponse` | last_online.go |
| `SendFriendRequest` | Gửi lời mời | `message,userID` | `SendFriendRequestResponse` | send_friend_request.go |
| `AcceptFriendRequest` | Chấp nhận | `friendID` | `AcceptFriendRequestResponse` | accept_friend_request.go |
| `RejectFriendRequest` | Từ chối | `friendID` | `RejectFriendRequestResponse` | reject_friend_request.go |
| `UndoFriendRequest` | Hủy lời mời đã gửi | `userID` | `UndoFriendRequestResponse` | undo_friend_request.go |
| `GetSentFriendRequest` | DS lời mời đã gửi | — | `*GetSentFriendRequestResponse` | get_sent_friend_request.go |
| `GetFriendRequestStatus` | Trạng thái lời mời | `friendID` | `*GetFriendRequestStatusResponse` | get_friend_request_status.go |
| `RemoveFriend` | Xóa bạn | `friendID` | `RemoveFriendResponse` | remove_friend.go |
| `BlockUser` | Chặn | `userID` | `BlockUserResponse` | block_user.go |
| `UnblockUser` | Bỏ chặn | `userID` | `UnblockUserResponse` | unblock_user.go |
| `SetViewFeedBlock` | Chặn xem nhật ký | `userID,bool` | `SetViewFeedBlockResponse` | set_view_feed_block.go |
| `UpdateAlias` | Đặt tên gợi nhớ | `userID,alias` | `UpdateAliasResponse` | update_alias.go |
| `RemoveAlias` | Xóa tên gợi nhớ | `userID` | `RemoveAliasResponse` | remove_alias.go |
| `GetAliasList` | DS tên gợi nhớ | `OffsetPaginationOptions` | `*GetAliasListResponse` | get_alias_list.go |
| `GetFriendBoardList` | Board của bạn | `friendID` | `*GetFriendBoardListResponse` | get_friend_board_list.go |

### 11.4 Nhóm

| Hàm Go | Mô tả | Req chính | Resp | File |
|---|---|---|---|---|
| `CreateGroup` | Tạo nhóm | `CreateGroupOptions` | `*CreateGroupResponse` | create_group.go |
| `DeleteGroup` | Giải tán nhóm | `groupID` | `DeleteGroupResponse` | delete_group.go |
| `LeaveGroup` | Rời nhóm | `groupID,bool` | `*LeaveGroupResponse` | leave_group.go |
| `AddUserToGroup` | Thêm thành viên | `groupID,...userID` | `*AddUserToGroupResponse` | add_user_to_group.go |
| `RemoveUserFromGroup` | Xóa thành viên | `groupID,...memberID` | `*RemoveUserFromGroupResponse` | remove_user_from_group.go |
| `InviteUserToGroups` | Mời vào nhiều nhóm | `userID,...groupID` | `*InviteUserToGroupsResponse` | invite_user_to_groups.go |
| `GetGroupInfo` | Thông tin nhóm | `...groupID` | `*GetGroupInfoResponse` | get_group_info.go |
| `GetAllGroups` | Tất cả nhóm | — | `*GetAllGroupsResponse` | get_all_groups.go |
| `GetGroupMembersInfo` | Thông tin thành viên | `...memberID` | `*GetGroupMembersInfoResponse` | get_group_members_info.go |
| `GetGroupChatHistory` | Lịch sử chat nhóm | `groupID,count` | `*GetGroupChatHistoryResponse` | get_group_chat_history.go |
| `UpdateGroupName` | Đổi tên nhóm | `groupID,name` | `*UpdateGroupNameResponse` | update_group_name.go |
| `UpdateGroupAvatar` | Đổi avatar nhóm | `groupID,model.AttachmentSource` | `UpdateGroupAvatarResponse` | update_group_avatar.go |
| `UpdateGroupSetting` | Cài đặt nhóm | `groupID,UpdateGroupSettingOptions` | `UpdateGroupSettingResponse` | update_group_setting.go |
| `UpgradeGroupToCommunity` | Nâng lên community | `groupID` | `UpgradeGroupToCommunityResponse` | upgrade_group_to_community.go |
| `ChangeGroupOwner` | Đổi chủ nhóm | `groupID,memberID` | `*ChangeGroupOwnerResponse` | change_group_owner.go |
| `AddGroupDeputy` | Thêm phó nhóm | `groupID,...memberID` | `AddGroupDeputyResponse` | add_group_deputy.go |
| `RemoveGroupDeputy` | Bỏ phó nhóm | `groupID,...memberID` | `RemoveGroupDeputyResponse` | remove_group_deputy.go |
| `EnableGroupLink` | Bật link nhóm | `groupID` | `*EnableGroupLinkResponse` | enable_group_link.go |
| `DisableGroupLink` | Tắt link nhóm | `groupID` | `DisableGroupLinkResponse` | disable_group_link.go |
| `GetGroupLinkDetail` | Chi tiết link nhóm | `groupID` | `*GetGroupLinkDetailResponse` | get_group_link_detail.go |
| `GetGroupLinkInfo` | Info nhóm từ link | `link,memberPage` | `*GetGroupLinkInfoResponse` | get_group_link_info.go |
| `JoinGroupLink` | Vào nhóm qua link | `link` | `JoinGroupLinkResponse` | join_group_link.go |
| `GetGroupPendingJoinRequests` | DS chờ duyệt | `groupID` | `*GetGroupPendingJoinRequestsResponse` | get_group_pending_join_requests.go |
| `ReviewPendingMemberRequest` | Duyệt thành viên | `groupID,ReviewPendingMemberData` | `*ReviewPendingMemberRequestResponse` | review_pending_member_request.go |
| `AddGroupBlockedMember` | Chặn thành viên | `groupID,...memberID` | `AddGroupBlockedMemberResponse` | add_group_blocked_member.go |
| `RemoveGroupBlockedMember` | Bỏ chặn thành viên | `groupID,...memberID` | `RemoveGroupBlockedMemberResponse` | remove_group_blocked_member.go |
| `GetGroupBlockedMember` | DS bị chặn trong nhóm | `groupID,OffsetPaginationOptions` | `*GetGroupBlockedMemberResponse` | get_group_blocked_member.go |
| `GetGroupBoardList` | DS board nhóm | `groupID,OffsetPaginationOptions` | `*GetGroupBoardListResponse` | get_group_board_list.go |
| `GetGroupInviteBoxList` | DS invite box | `GetGroupInviteBoxListOptions` | `*GetGroupInviteBoxListResponse` | get_group_invite_box_list.go |
| `GetGroupInviteBoxInfo` | Info invite box | `groupID,OffsetPaginationOptions` | `*GetGroupInviteBoxInfoResponse` | get_group_invite_box_info.go |
| `JoinGroupInviteBox` | Vào nhóm qua invite box | `groupID` | `JoinGroupInviteBoxResponse` | join_group_invite_box.go |
| `RemoveGroupInviteBox` | Xóa lời mời nhóm | `bool,...groupID` | `*RemoveGroupInviteBoxResponse` | remove_group_invite_box.go |

### 11.5 Tài khoản, avatar, settings, biz

| Hàm Go | Mô tả | Req chính | Resp | File |
|---|---|---|---|---|
| `GetAccountInfo` | Thông tin tài khoản | — | `*GetAccountInfoResponse` | get_account_info.go |
| `GetSetting` | Cài đặt hiện tại | — | `*GetSettingResponse` | get_setting.go |
| `UpdateSetting` | Đổi 1 cài đặt | `UpdateSettingType,int` | `UpdateSettingResponse` | update_setting.go |
| `UpdateProfile` | Cập nhật hồ sơ | `UpdateProfileData` | `UpdateProfileResponse` | update_profile.go |
| `UpdateProfileBio` | Cập nhật bio/status | `bio` | `UpdateProfileBioResponse` | update_profile_bio.go |
| `UpdateAccountAvatar` | Đổi avatar | `model.AttachmentSource` | `UpdateAccountAvatarResponse` | update_account_avatar.go |
| `UpdateActiveStatus` | Trạng thái hoạt động | `bool` | `*UpdateActiveStatusResponse` | update_active_status.go |
| `UpdateLanguage` | Đổi ngôn ngữ | `Language` | `UpdateLanguageResponse` | update_language.go |
| `GetAvatarList` | DS avatar | `OffsetPaginationOptions` | `*GetAvatarListResponse` | get_avatar_list.go |
| `GetAvatarUrlProfile` | URL avatar | `avatarSize,...friendID` | `*GetAvatarUrlProfileResponse` | get_avatar_url_profile.go |
| `GetFullAvatar` | Avatar đầy đủ | `friendID` | `*GetFullAvatarResponse` | get_full_avatar.go |
| `ReuseAvatar` | Dùng lại avatar | `photoID` | `*ReuseAvatarResponse` | reuse_avatar.go |
| `DeleteAvatar` | Xóa avatar | `...photoID` | `*DeleteAvatarResponse` | delete_avatar.go |
| `GetQR` | QR danh thiếp | `...userID` | `*GetQRResponse` | get_qr.go |
| `GetCookie` | Lấy cookie hiện tại | — | `GetCookieResponse` | get_cookie.go |
| `KeepAlive` | Giữ kết nối | — | `*KeepAliveResponse` | keep_alive.go |
| `GetBizAccount` | Tài khoản biz | `friendID` | `*GetBizAccountResponse` | get_biz_account.go |
| `GetAutoReplyList` | DS auto reply | — | `*GetAutoReplyListResponse` | get_auto_reply_list.go |
| `CreateAutoReply` | Tạo auto reply | `CreateAutoReplyRequest` | `*CreateAutoReplyResponse` | create_auto_reply.go |
| `UpdateAutoReply` | Sửa auto reply | `UpdateAutoReplyRequest` | `*UpdateAutoReplyResponse` | update_auto_reply.go |
| `DeleteAutoReply` | Xóa auto reply | `id int` | `*DeleteAutoReplyResponse` | delete_auto_reply.go |

### 11.6 Poll / Note / Reminder / Quick-message

| Hàm Go | Mô tả | Req chính | Resp | File |
|---|---|---|---|---|
| `CreatePoll` | Tạo poll nhóm | `groupID,CreatePollOptions` | `*CreatePollResponse` | create_poll.go |
| `VotePoll` | Bình chọn | `pollID,...optionID` | `*VotePollResponse` | vote_poll.go |
| `AddPollOptions` | Thêm lựa chọn | `AddPollOptionsRequest` | `*AddPollOptionsResponse` | add_poll_options.go |
| `GetPollDetail` | Chi tiết poll | `pollID int` | `*GetPollDetailResponse` | get_poll_detail.go |
| `LockPoll` | Khóa poll | `pollID int` | `LockPollResponse` | lock_poll.go |
| `SharePoll` | Chia sẻ poll | `pollID int` | `SharePollResponse` | share_poll.go |
| `CreateNote` | Tạo note nhóm | `groupID,CreateNoteOptions` | `*CreateNoteResponse` | create_note.go |
| `UpdateNote` | Sửa note | `groupID,UpdateNoteOptions` | `*UpdateNoteResponse` | update_note.go |
| `CreateReminder` | Tạo nhắc hẹn | `threadID,TT,CreateReminderOptions` | `*CreateReminderResponse` | create_reminder.go |
| `UpdateReminder` | Sửa nhắc hẹn | `threadID,TT,UpdateReminderOptions` | `*UpdateReminderResponse` | update_reminder.go |
| `RemoveReminder` | Xóa nhắc hẹn | `threadID,TT,reminderID` | `RemoveReminderResponse` | remove_reminder.go |
| `GetReminder` | Chi tiết nhắc hẹn | `reminderID` | `*GetReminderResponse` | get_reminder.go |
| `GetReminderList` | DS nhắc hẹn | `threadID,TT,OffsetPaginationOptions` | `*GetReminderListResponse` | get_reminder_list.go |
| `GetReminderResponse` | Phản hồi nhắc hẹn | `reminderID` | `*GetReminderResponseResponse` | get_reminder_response.go |
| `AddQuickMessage` | Thêm tin nhanh | `AddQuickMessageRequest` | `*AddQuickMessageResponse` | add_quick_message.go |
| `UpdateQuickMessage` | Sửa tin nhanh | `itemID,UpdateQuickMessageData` | `*UpdateQuickMessageResponse` | update_quick_message.go |
| `RemoveQuickMessage` | Xóa tin nhanh | `...messageID int` | `*RemoveQuickMessageResponse` | remove_quick_message.go |
| `GetQuickMessageList` | DS tin nhanh | — | `*GetQuickMessageListResponse` | get_quick_message_list.go |

### 11.7 Sticker & Catalog (zBusiness)

| Hàm Go | Mô tả | Req chính | Resp | File |
|---|---|---|---|---|
| `GetStickers` | Tìm sticker ID theo keyword | `keyword` | `*GetStickersResponse` | get_stickers.go |
| `GetStickerDetail` | Chi tiết 1 sticker | `stickerID int` | `*GetStickerDetailResponse` | get_sticker_detail.go |
| `GetStickersDetail` | Chi tiết nhiều sticker | `...stickerID int` | `*GetStickersDetailResponse` | get_stickers_detail.go |
| `SearchSticker` | Tìm sticker | `keyword,limit` | `*SearchStickerResponse` | search_sticker.go |
| `GetStickerCategoryDetail` | Chi tiết category | `cateID int` | `*GetStickerCategoryDetailResponse` | get_sticker_category_detail.go |
| `GetCatalogList` | DS catalog | `GetCatalogListRequest` | `*GetCatalogListResponse` | get_catalog_list.go |
| `CreateCatalog` | Tạo catalog | `name` | `*CreateCatalogResponse` | create_catalog.go |
| `UpdateCatalog` | Sửa catalog | `UpdateCatalogRequest` | `*UpdateCatalogResponse` | update_catalog.go |
| `DeleteCatalog` | Xóa catalog | `catalogID` | `DeleteCatalogResponse` | delete_catalog.go |
| `CreateProductCatalog` | Tạo sản phẩm | `CreateProductCatalogRequest` | `*CreateProductCatalogResponse` | create_product_catalog.go |
| `GetProductCatalogList` | DS sản phẩm | `GetProductCatalogListRequest` | `*GetProductCatalogListResponse` | get_product_catalog_list.go |
| `UpdateProductCatalog` | Sửa sản phẩm | `UpdateProductCatalogRequest` | `*UpdateProductCatalogResponse` | update_product_catalog.go |
| `DeleteProductCatalog` | Xóa sản phẩm | `DeleteProductCatalogRequest` | `*DeleteProductCatalogResponse` | delete_product_catalog.go |

### 11.8 Helper (không phải REST API)

| Hàm Go | Mô tả | File |
|---|---|---|
| `GetContext() (session.Context, error)` | Lấy session context | get_context.go |
| `GetOwnID() string` | ID tài khoản đang đăng nhập | get_own_id.go |
| `Listener() listener.Listener` | Lấy listener WebSocket | listener.go |

---

## 12. Mã lỗi & xử lý lỗi

Package `errs` định nghĩa 2 loại lỗi chính:

### 12.1 `errs.ZaloAPIError` — lỗi từ phía Zalo (upstream)

Khi Zalo trả về `meta.code != 0`, thư viện bọc thành `ZaloAPIError{Code, Message}`.

```go
resp, err := a.SendMessage(ctx, userID, model.ThreadTypeUser, msg)
if err != nil {
    var apiErr errs.ZaloAPIError
    if errors.As(err, &apiErr) {
        if apiErr.Code != nil {
            log.Printf("Zalo lỗi [%d]: %s", *apiErr.Code, apiErr.Message)
        }
    }
}
```

Một số mã lỗi Zalo hay gặp (ghi trong doc-comment ở `apis.go`):

| Code | Ngữ cảnh |
|---|---|
| 114 | Tham số không hợp lệ (`errs.ZaloErrorCodeInvalidParams`) |
| 112 | Không có lời mời kết bạn (`GetSentFriendRequest`) |
| 165 / 166 | Không có quyền / không phải thành viên nhóm |
| 178 | Đã là thành viên nhóm (`JoinGroupLink`) |
| 212 | Mục không tồn tại (quick message) |
| 215 / 222 / 225 | Bị chặn / đã có lời mời ngược / đã là bạn (`SendFriendRequest`) |
| 240 | Nhóm cần duyệt thành viên (`JoinGroupLink`) |
| 821 | Vượt giới hạn quick message |

### 12.2 `errs.ZCAError` — lỗi nội bộ thư viện

Lỗi do client (sai tham số, thiếu cấu hình, mạng, mã hóa...). Các sentinel error hay dùng:

| Lỗi | Khi nào |
|---|---|
| `errs.ErrMissingImageMetadataGetter` | Gửi ảnh/file mà chưa set `WithImageMetadataGetter` |
| `errs.ErrSourceEmpty` | Đính kèm rỗng |
| `errs.ErrExceedMaxFile` / `ErrExceedMaxFileSize` | Quá số lượng / dung lượng file |
| `errs.ErrInvalidExtension` | Đuôi file không hợp lệ |
| `errs.ErrInvalidMessageCount` / `ErrInconsistentGroupRecipient` | `SendSeenEvent`/`SendDeliveredEvent` sai |
| `errs.ErrLoginQRAborted` / `ErrLoginQRDeclined` | Hủy/từ chối khi login QR |
| `api.ErrPhoneNumberEmpty` | `FindUser` không có số |
| `api.ErrInvalidReaction` | `AddReaction` với reaction rỗng |
| `api.ErrMessageContentEmpty`, `api.ErrInvalidMention` | `SendMessage` sai nội dung/mention |

```go
if errors.Is(err, errs.ErrMissingImageMetadataGetter) {
    // cần cấu hình WithImageMetadataGetter trước khi gửi ảnh
}
```

### 12.3 Bot offline / mất kết nối listener

- Listener đóng kết nối → đọc được trên `ln.Closed()` (kiểu `CloseInfo`). Mã đóng tham khảo (hằng trong
  `listener/listener.go`): `1000` đóng thủ công, `1006` bất thường, `3000` trùng kết nối, `3003` bị kick.
- Đặt `Start(ctx, true)` để **tự kết nối lại** theo cấu hình retry của Zalo. Khi `Closed()` phát tín hiệu
  nghĩa là đã hết retry → cần đăng nhập/khởi động lại.
- Nếu Zalo bị lỗi phía server, REST call trả `ZaloAPIError`; nên có retry/backoff ở tầng gọi.

---

## 13. Ghi chú khác biệt so với upstream

- **Module độc lập:** import nội bộ `github.com/luannt2002/zalo-api-unofficial/...`. **KHÔNG** dùng `github.com/amrakk/zcago`.
  README và tài liệu bản tách dùng module mới; không import đường dẫn upstream Go cũ.
- **Kiến trúc endpoint kiểu factory:** mỗi API là 1 file `api/*.go` gồm struct request/response, một
  `XxxFn` (alias hàm), một `xxxFactory` (khởi tạo endpoint), và method mỏng `(a *api) Xxx(...)` ủy quyền
  cho `a.e.Xxx`. Tất cả method bind trong `api/api.go` (`initEndpoints`).
- **Video/Voice gửi bằng URL** (`SendVideoOptions.VideoURL`, `SendVoiceOptions.VoiceURL`) thay vì upload
  file trực tiếp như một số luồng khác.
- **GIF** có luồng riêng (`SendGIF`) hoặc đính kèm `.gif` trong `SendMessage` (tự tách & upload thumbnail).
- **Một số API fork tự thêm / mở rộng** so với mô tả gốc, ví dụ: `SendBankCard` (kèm bảng `model.BinBankCard`
  cho BIN ngân hàng VN), `GetReminder*` (reminder cho cả user lẫn group), nhóm catalog/sản phẩm zBusiness,
  `GetGroupInviteBox*` / `JoinGroupInviteBox`, `UpgradeGroupToCommunity`, `SetViewFeedBlock`,
  `Update/Reset HiddenChatPIN`.
- **Một số hàm đang đánh dấu `@TODO: test`** trong doc-comment ở `apis.go` (ví dụ `ForwardMessage`,
  `CreateAutoReply`, `SendSeenEvent`, `SendDeliveredEvent`) — dùng thận trọng.
- **WithProxy** giúp tách egress IP cho nhiều bot (tránh ban IP) — phù hợp use-case multi-account.

---

## 14. Tham khảo

- Tài liệu gốc zca-js (TypeScript): <https://tdung.gitbook.io/zca-js>
- Mã nguồn fork: thư mục `api/`, `session/`, `listener/`, `model/`, `errs/` của `github.com/luannt2002/zalo-api-unofficial`.
- Bề mặt API đầy đủ (chữ ký + doc-comment): `apis.go` (interface `API`).
