# API Reference — zca-go (github.com/luannt2002/zalo-api-unofficial)

> Tài liệu TỪNG hàm API: gọi Zalo THẬT hay chỉ đọc state (instance/local), service zpw, endpoint + method,
> input bắt buộc/optional, output, ghi chú (mã hóa, chờ WS, metadata bắt buộc…). Sinh từ đọc code thật 2026-07-10.
> Xem [TEST_COVERAGE.md](./TEST_COVERAGE.md) cho kết quả test với bot thật + bug đã fix.

## Khuôn mẫu chung
Hầu hết hàm theo `apiFactory`: base = `sc.GetZpwService("<service>")` → `u.MakeURL` → `params = EncodeAES(jsonx.Stringify(payload))`
→ `u.Request` → `u.Resolve(resp, true)`. GET truyền `params` qua **query-string**; POST qua **form body** (`httpx.BuildFormBody`) hoặc **multipart** (upload).
`imei = sc.IMEI()` gần như luôn có trong payload. **Loại "instance/local"** = KHÔNG gọi HTTP (chỉ đọc session).

**Service zpw thường gặp:** `chat`(tin user), `group`(nhóm), `file`(media/upload/forward), `profile`(hồ sơ/avatar/mute),
`friend`(bạn bè), `conversation`(pin/hidden/unread/autodelete), `label`(archive/label), `catalog`, `sticker`,
`group_board`(note/reminder), `quick_message`, `auto_reply`, `alias`, `reaction`, `zimsg`(bankcard).

---

## 1. Gửi tin nhắn & media

| Hàm | Loại | Service | Endpoint (Method) | Input | Output | Ghi chú |
|---|---|---|---|---|---|---|
| SendMessage | Zalo REAL | text `chat`/`group`; media `file` | text: `/api/message/sms`·`/sendmsg`·`/mention`·`/quote`, group `/api/group/*`; ảnh `/api/message/photo_original/send`, video/file `/asyncfile/msg`, gif `/gif` (POST, `?nretry=0`) | threadID, threadType, MessageContent{Msg, Style[], Urgency, Quote, Mentions[], Attachments[], TTL}. Ảnh/video/file upload qua UploadAttachment trước | SendMessageResponse{Message, Attachment[] (MsgID)} | **Ký URL SAU khi nối path** (base includeDefaults=false → tránh 404). `flexID` msgId chuỗi/số. 1 ảnh + text → text làm desc. Gửi song song errgroup |
| SendSticker | Zalo REAL | `chat`/`group` | `/api/message/sticker`·`/api/group/sticker` (POST) | threadID, threadType, {ID, CateID, Type} | {MsgID string} | payload zsource=101 |
| SendGIF | Zalo REAL | `file` | `/api/message/gif`·`/api/group/gif` (POST multipart, chunk) | threadID, threadType, GIFContent{Attachment(file path/bytes), Thumb?, TTL} | {MsgID, Href} | Tự decode frame đầu → PNG thumb (UploadThumbnail) nếu thiếu; checksum MD5; chunk 220 = bỏ qua |
| SendVideo | Zalo REAL | `file` | `/api/message/forward`·`/api/group/forward` (POST) | threadID, threadType, {Msg, **VideoURL public**, **ThumbnailURL**, **Duration ms**, **Width**, **Height**, TTL} | {MsgID int} | **BẮT BUỘC thumbUrl+duration+width+height** (thiếu⇒114). HEAD lấy fileSize. key `toId` camelCase. zsource=704, msgType=5 |
| SendVoice | Zalo REAL | `file` | `/api/message/forward`·`/api/group/forward` (POST) | threadID, threadType, {**VoiceURL public** (m4a/aac), TTL} | {MsgID int} | HEAD lấy fileSize. key **`toId`** (dùng `toid`⇒111). zsource=-1, msgType=3 |
| SendCard | Zalo REAL | `file` | `/api/message/forward`·`/api/group/forward` (POST) | threadID, threadType, {UserID, PhoneNumber?, TTL} | {MsgID int} | Gọi `GetQR(UserID)` lấy qrCodeUrl. msgType=6 |
| SendBankCard | Zalo REAL | `zimsg` | `/api/transfer/card` (POST) | threadID, threadType, {BinBank, NumAccBank, NameAccBank} | {MsgID flexID} | **UnmarshalJSON linh hoạt**: data = string-trần HOẶC object {msgId} |
| SendLink | Zalo REAL | `chat`/`group` | `/api/message/link`·`/api/group/sendlink` (POST) | threadID, threadType, {Msg, Link, TTL, Mentions[]} | {MsgID flexID} | Gọi `ParseLink(Link)` lấy title/desc/thumb. type=2 |
| SendReport | Zalo REAL | `profile` | user `/api/report/abuse-v2`, group `/api/social/profile/reportabuse` (POST) | threadID, threadType, {Content, Reason (0 Other,1 Sensitive,2 Annoying,3 Fraud)} | {ReportID} | ⚠️ tố cáo — cẩn thận |
| SendTypingEvent | Zalo REAL | `chat`/`group` | `/api/message/typing`·`/api/group/typing` (POST) | threadID, threadType, destType | {Status} | UnmarshalJSON: response có thể là string |
| SendSeenEvent | Zalo REAL | `chat`/`group` | `/api/message/seenv2`·`/api/group/seenv2` (POST) | messages[], threadType | {Status} | cùng thread (ErrInconsistentGroupRecipient) |
| SendDeliveredEvent | Zalo REAL | `chat`/`group` | `/api/message/deliveredv2`·`/api/group/deliveredv2` (POST) | messages[], threadType, isSeen | string/{status} | giới hạn số msg/request |
| ForwardMessage | Zalo REAL | `file` | `/api/message/mforward`·`/api/group/mforward` (POST) | threadIDs[], threadType, {Message, TTL, Reference} | {success[], fail[]} | user `toIds`+imei, group `grids` |
| UndoMessage | Zalo REAL | `chat`/`group` | `/api/message/undo`·`/api/group/undomsg` (POST) | threadID, threadType, {MsgID, CliMsgID} | {Status} | thu hồi tin bot đã gửi |
| DeleteMessage | Zalo REAL | `chat`/`group` | `/api/message/delete`·`/api/group/deletemsg` (POST) | dest{ThreadID,Type,Data{CliMsgID,MsgID,UIDFrom}}, onlyMe | {Status} | tự-recall dùng UndoMessage; DM không xóa-cho-mọi-người |
| AddReaction | Zalo REAL | `reaction` | `/api/message/reaction`·`/api/group/reaction` (POST) | dest{ThreadID,Type,Data{MsgID,CliMsgID}}, reaction{RIcon,RType,Source} | {MsgIDs[]} | validate reaction.IsValid() |
| UploadAttachment | Zalo REAL | `file` | ảnh `/photo_original/upload`, video/file `/asyncfile/upload`, prefix `/message`·`/group` (POST multipart chunk) | threadID, threadType, sources...(file path/bytes) | []UploadAttachment{Image/File} | **TotalChunk = ceil(Size/chunkSize)** (Size=0⇒fail). Video/file **ASYNC chờ WS `file_done`**; ảnh trả trực tiếp |
| UploadPhoto | Zalo REAL | `file` | `/api/product/upload/photo` (POST multipart) | source (file path/bytes) | {NormalURL,PhotoID,HdURL,ThumbURL…} | single chunk; toid=Send2meID |
| UploadThumbnail | Zalo REAL | `file` | `/api/message/upthumb` (POST multipart) | source | {HDURL,URL,FileID} | KHÔNG cần size/w/h; field `fileContent` image/png |

## 2. Bạn bè (friend)

| Hàm | Loại | Service | Endpoint (Method) | Input | Output | Ghi chú |
|---|---|---|---|---|---|---|
| SendFriendRequest | Zalo REAL | `friend` | `/api/friend/sendreq` (POST) | message, userID | string | reqsrc=30 |
| AcceptFriendRequest | Zalo REAL | `friend` | `/api/friend/accept` (POST) | friendID | string | |
| RejectFriendRequest | Zalo REAL | `friend` | `/api/friend/reject` (POST) | friendID | string | |
| UndoFriendRequest | Zalo REAL | `friend` | `/api/friend/undo` (POST) | userID | string | không gửi imei |
| RemoveFriend | Zalo REAL | `friend` | `/api/friend/remove` (POST) | friendID | string | ⚠️ hủy kết bạn |
| BlockUser | Zalo REAL | `friend` | `/api/friend/block` (POST) | userID | string | |
| UnblockUser | Zalo REAL | `friend` | `/api/friend/unblock` (POST) | userID | string | |
| SetViewFeedBlock | Zalo REAL | `friend` | `/api/friend/feed/block` (POST) | userID, isBlock | string | |
| GetFriendRecommendations | Zalo REAL | `friend` | `/api/friend/recommendsv2/list` (GET) | — | {recommItems[]} | |
| GetFriendRequestStatus | Zalo REAL | `friend` | `/api/friend/reqstatus` (GET) | friendID | {is_friend,is_requested,is_requesting} | |
| GetSentFriendRequest | Zalo REAL | `friend` | `/api/friend/requested/list` (GET) | — | map[id]info | |
| GetAllFriends | Zalo REAL | `profile` | `/api/social/friend/getfriends` (GET) | {Page,Count def 20000} | []User | |
| GetCloseFriends | Zalo REAL | `profile` | `/api/social/friend/getclosedfriends` (GET) | — | []User | ⚠️ **Zalo ĐÃ BỎ → 404** |
| GetFriendOnlineStatus | Zalo REAL | `profile` | `/api/social/friend/onlines` (GET) | — | {onlines[]} | ⚠️ **Zalo ĐÃ BỎ → 404** |
| FindUser | Zalo REAL | `friend` | `/api/friend/profile/multiget` (GET) | phoneNumber...(≥1) | map[phone]UserBasic | |
| FindUserByUsername | Zalo REAL | `friend` | `/api/friend/search/by-user-name` (GET) | username | UserBasic | |
| GetMultiUsersByPhones | Zalo REAL | `friend` | `/api/friend/profile/multiget` (GET) | phoneNumbers[] | map[phone]UserBasic | chuẩn hóa 0→84 |
| GetRelatedFriendGroup | Zalo REAL | `friend` | `/api/friend/group/related` (POST) | friendID... | {GroupRelateds} | |
| GetQR | Zalo REAL | `friend` | `/api/friend/mget-qr` (POST) | userID... | map[id]qrUrl | |
| GetFriendBoardList | Zalo REAL | `friend_board` | `/api/friendboard/list` (GET) | friendID | {data[]} | ghim hội thoại 1-1 |
| GetAliasList | Zalo REAL | `alias` | `/api/alias/list` (GET) | {Page,Count} | {items[]} | |
| UpdateAlias | Zalo REAL | `alias` | `/api/alias/update` (GET) | userID, alias | string | biệt danh LOCAL (bot view) |
| RemoveAlias | Zalo REAL | `alias` | `/api/alias/remove` (GET) | userID | string | |

## 3. Nhóm (group)

| Hàm | Loại | Service | Endpoint (Method) | Input | Output | Ghi chú |
|---|---|---|---|---|---|---|
| CreateGroup | Zalo REAL | `group` | `/api/group/create/v2` (POST) | {Name?, Members[] bắt buộc} | {GroupID,SuccessMembers,ErrorMembers} | lỗi nếu Members rỗng |
| DeleteGroup | Zalo REAL | `group` | `/api/group/disperse` (POST) | groupID | string | giải tán nhóm |
| LeaveGroup | Zalo REAL | `group` | `/api/group/leave` (POST) | groupID, isSilent | {MemberError[]} | 1 group/lần |
| AddUserToGroup | Zalo REAL | `group` | `/api/group/invite/v2` (POST) | groupID, userID... | {ErrorMembers,ErrorData} | |
| RemoveUserFromGroup | Zalo REAL | `group` | `/api/group/kickout` (POST) | groupID, memberID... | {ErrorMembers[]} | |
| ChangeGroupOwner | Zalo REAL | `group` | `/api/group/change-owner` (GET) | groupID, memberID | {Time} | |
| AddGroupDeputy | Zalo REAL | `group` | `/api/group/admins/add` (GET) | groupID, memberID... | string | |
| RemoveGroupDeputy | Zalo REAL | `group` | `/api/group/admins/remove` (GET) | groupID, memberID... | string | |
| AddGroupBlockedMember | Zalo REAL | `group` | `/api/group/blockedmems/add` (GET) | groupID, memberID... | string | |
| RemoveGroupBlockedMember | Zalo REAL | `group` | `/api/group/blockedmems/remove` (GET) | groupID, memberID... | string | |
| GetGroupBlockedMember | Zalo REAL | `group` | `/api/group/blockedmems/list` (GET) | groupID, {Count,Page} | {blocked_members[],has_more} | |
| InviteUserToGroups | Zalo REAL | `group` | `/api/group/invite/multi` (GET) | userID, groupID... | {GridMessageMap} | |
| GetGroupInfo | Zalo REAL | `group` | `/api/group/getmg-v2` (POST) | groupID... | {GridInfoMap} | |
| GetAllGroups | Zalo REAL | `group_poll` | `/api/group/getlg/v4` (GET) | — | {version, gridVerMap} | không EncodeAES |
| GetGroupMembersInfo | Zalo REAL | `profile` | `/api/social/group/members` (GET) | memberID... | {Profiles} | service=profile |
| GetGroupChatHistory | Zalo REAL | `group` | `/api/group/history` (GET) | groupID, count def 50 | {GroupMsgs[]} | |
| GetGroupBoardList | Zalo REAL | `group_board` | `/api/board/list` (GET) | groupID, {Count,Page} | {items[] (Poll/Note/Pin)} | UnmarshalJSON theo boardType |
| GetGroupPendingJoinRequests | Zalo REAL | `group` | `/api/group/pending-mems/list` (GET) | groupID | {Users[],Time} | |
| ReviewPendingMemberRequest | Zalo REAL | `group` | `/api/group/pending-mems/review` (GET) | groupID, {Members[], IsApprove} | map[id]Status | |
| EnableGroupLink | Zalo REAL | `group` | `/api/group/link/new` (GET) | groupID | {link,expiration_date,enabled} | |
| DisableGroupLink | Zalo REAL | `group` | `/api/group/link/disable` (GET) | groupID | string | |
| GetGroupLinkDetail | Zalo REAL | `group` | `/api/group/link/detail` (GET) | groupID | {Link,ExpirationDate,Enabled} | |
| GetGroupLinkInfo | Zalo REAL | `group` | `/api/group/link/ginfo` (GET) | link, memberPage | {GroupID,Name,AdminIDs…} | |
| JoinGroupLink | Zalo REAL | `group` | `/api/group/link/join` (GET) | link | string | |
| JoinGroupInviteBox | Zalo REAL | `group` | `/api/group/inv-box/join` (GET) | groupID | string | |
| GetGroupInviteBoxInfo | Zalo REAL | `group` | `/api/group/inv-box/inv-info` (GET) | groupID, {Count,Page} | {GroupInfo,InviterInfo…} | |
| GetGroupInviteBoxList | Zalo REAL | `group` | `/api/group/inv-box/list` (GET) | {MPage,InvPerPage…} | {Invitations[],Total,HasMore} | |
| RemoveGroupInviteBox | Zalo REAL | `group` | `/api/group/inv-box/mdel-inv` (GET) | blockFutureInvite, groupID... | {DelInvitationIds,ErrMap} | |
| UpgradeGroupToCommunity | Zalo REAL | `group` | `/api/group/upgrade/community` (GET) | groupID | string | |
| UpdateGroupName | Zalo REAL | `group` | `/api/group/updateinfo` (POST) | groupID, name | {Status} | |
| UpdateGroupAvatar | Zalo REAL | `file` | `/api/group/upavatar` (POST multipart) | groupID, source(file/bytes) | string | field `fileContent` image/jpeg |
| UpdateGroupSetting | Zalo REAL | `group` | `/api/group/setting/update` (GET) | groupID, {9 cờ bool} | string | |

## 4. Poll (bình chọn nhóm)

| Hàm | Loại | Service | Endpoint (Method) | Input | Output | Ghi chú |
|---|---|---|---|---|---|---|
| CreatePoll | Zalo REAL | `group` | `/api/poll/create` (POST) | groupID, {Question,Options[],ExpiredTime,Allow*,Is*} | PollDetail | |
| VotePoll | Zalo REAL | `group` | `/api/poll/vote` (GET) | pollID, optionID... (rỗng=unvote) | {Options[]} | |
| AddPollOptions | Zalo REAL | `group` | `/api/poll/option/add` (GET) | {PollId,Options[],VotedOptionIds[]} | {Options[]} | |
| LockPoll | Zalo REAL | `group` | `/api/poll/end` (POST) | pollID | string | |
| SharePoll | Zalo REAL | `group` | `/api/poll/share` (POST) | pollID | string | |
| GetPollDetail | Zalo REAL | `group` | `/api/poll/detail` (POST) | pollID | PollDetail | |

## 5. Hồ sơ / Tài khoản / Avatar

| Hàm | Loại | Service | Endpoint (Method) | Input | Output | Ghi chú |
|---|---|---|---|---|---|---|
| GetAccountInfo | Zalo REAL | `profile` | `/api/social/profile/me-v2` (GET) | — | {profile:User} | không EncodeAES |
| GetUserInfo | Zalo REAL | `profile` | `/api/social/friend/getprofiles/v2` (POST) | userID... (tự thêm `_0`) | {ChangedProfiles} | |
| GetUserSummaryInfo | Zalo REAL | `profile` | `/api/social/group/members` (GET) | userID... | {Profiles} | |
| GetBizAccount | Zalo REAL | `profile` | `/api/social/friend/get-bizacc` (POST) | friendID | {biz,setting_start_page} | |
| LastOnline | Zalo REAL | `profile` | `/api/social/profile/lastOnline` (GET) | userID | {ShowOnlineStatus,LastOnline} | |
| UpdateProfile | Zalo REAL | `profile` | `/api/social/profile/update` (POST) | {Profile{Name,DOB,Gender},Biz{…}} | string | ⚠️ đổi hồ sơ bot |
| UpdateProfileBio | Zalo REAL | `profile` | `/api/social/profile/status` (POST) | bio | string | |
| UpdateAccountAvatar | Zalo REAL | `file` | `/api/profile/upavatar` (POST multipart) | source(file/bytes) | string | ⚠️ đổi avatar bot |
| GetAvatarList | Zalo REAL | `profile` | `/api/social/avatar-list` (GET) | {Count,Page} | {photos[]} | |
| GetAvatarUrlProfile | Zalo REAL | `profile` | `/api/social/profile/avatar-url` (GET) | avatarSize, friendID... | map[id]{avatar} | |
| GetFullAvatar | Zalo REAL | `profile` | `/api/social/profile/avatar` (GET) | friendID | {full_avatar} | |
| DeleteAvatar | Zalo REAL | `profile` | `/api/social/del-avatars` (GET) | photoID... | {DelPhotoIds} | |
| ReuseAvatar | Zalo REAL | `profile` | `/api/social/reuse-avatar` (GET) | photoID | (null) | |
| UpdateActiveStatus | Zalo REAL | `profile` | `/api/social/profile/ping`·`/deactive` (GET) | isActive | {Status} | |
| UpdateLanguage | Zalo REAL | `profile` | `/api/social/profile/updatelang` (GET) | lang (VI/EN) | string | |
| GetSetting | Zalo REAL | hard-code | `wpa.chat.zalo.me/api/setting/me` (GET) | — | UserSetting | **URL cứng** |
| UpdateSetting | Zalo REAL | hard-code | `wpa.chat.zalo.me/api/setting/update` (GET) | sType, value | string | **URL cứng** |
| GetOwnID | instance/local | — | — | — | string (UID) | trả sc.UID(), không HTTP |
| GetContext | instance/local | — | — | — | session.Context | không HTTP |
| GetCookie | instance/local | — | — | — | http.CookieJar | không HTTP |
| KeepAlive | Zalo REAL | `chat` | `/keepalive` (GET) | — | {ConfigVesion} | giữ phiên |
| ParseLink | Zalo REAL | `file` | `/api/message/parselink` (GET) | link | {Thumb,Title,Desc,Href…} | dùng bởi SendLink |

## 6. Catalog / Sản phẩm (⚠️ cần tài khoản OA — bot cá nhân = FEATURE_NOT_ENABLED)

| Hàm | Service | Endpoint (Method) | Input | Output |
|---|---|---|---|---|
| CreateCatalog | `catalog` | `/api/prodcatalog/catalog/create` (POST) | name | {Item,Version*} |
| UpdateCatalog | `catalog` | `/api/prodcatalog/catalog/update` (POST) | {CatalogID,CatalogName} | {Item} |
| DeleteCatalog | `catalog` | `/api/prodcatalog/catalog/delete` (POST) | catalogID | string |
| GetCatalogList | `catalog` | `/api/prodcatalog/catalog/list` (POST) | {Limit,Page} | {items[],has_more} |
| CreateProductCatalog | `catalog` | `/api/prodcatalog/product/create` (POST) | {CatalogID,ProductName,Price,Files[]…} | {Item} | (upload ≤5 ảnh qua UploadProductPhoto) |
| UpdateProductCatalog | `catalog` | `/api/prodcatalog/product/update` (POST) | {ProductID,…,Files[]} | {Item} |
| DeleteProductCatalog | `catalog` | `/api/prodcatalog/product/mdelete` (POST) | {ProductIDs[],CatalogID} | {Item[]} |
| GetProductCatalogList | `catalog` | `/api/prodcatalog/product/list` (POST) | {CatalogID,Limit,Page} | {Items[],has_more} |
| UploadProductPhoto | `file` | `/api/product/upload/photo` (POST multipart) | source | {NormalURL,PhotoID,HdURL…} |

## 7. Reminder / Note (group_board)

| Hàm | Service | Endpoint (Method) | Input | Output | Ghi chú |
|---|---|---|---|---|---|
| CreateReminder | `group_board` | user `/api/board/oneone/create`, group `/api/board/topic/createv2` (POST) | threadID, threadType, {Title,Emoji,StartTime,Repeat} | ReminderResponse | ⚠️ recipient nhận thông báo |
| UpdateReminder | `group_board` | user `/api/board/oneone/update`, group `/api/board/topic/updatev2` (POST) | threadID, threadType, {Title,TopicId,Emoji,StartTime,Repeat} | ReminderResponse | |
| RemoveReminder | `group_board` | user `/api/board/oneone/remove`, group `/api/board/topic/remove` (POST) | threadID, threadType, reminderID | string | |
| GetReminder | `group_board` | `/api/board/topic/getReminder` (GET) | reminderID | ReminderGroup | |
| GetReminderList | `group_board` | user `/api/board/oneone/list`, group `/api/board/listReminder` (GET) | threadID, threadType, {Count,Page} | ReminderResponse | |
| GetReminderResponse | `group_board` | `/api/board/topic/listResponseEvent` (GET) | reminderID | {AcceptMember,RejectMember} | |
| CreateNote | `group_board` | `/api/board/topic/createv2` (POST) | groupID, {Title,PinAct} | NoteDetail | |
| UpdateNote | `group_board` | `/api/board/topic/updatev2` (POST) | groupID, {Title,TopicId,PinAct} | NoteDetail | |

## 8. Sticker

| Hàm | Service | Endpoint (Method) | Input | Output |
|---|---|---|---|---|
| GetStickers | `sticker` | `/api/message/sticker/suggest/stickers` (GET) | keyword | StickerSuggestions |
| SearchSticker | `sticker` | `/api/message/sticker/search` (GET) | keyword, limit def 50 | []StickerBasic |
| GetStickerDetail | `sticker` | `/api/message/sticker/sticker_detail` (GET) | stickerID | StickerDetail |
| GetStickersDetail | `sticker` | `/api/message/sticker/sticker_detail` (GET) | stickerID... | []StickerDetail (fan-out) |
| GetStickerCategoryDetail | `sticker` | `/api/message/sticker/category/sticker_detail` (GET) | cateID | []StickerDetail |

## 9. Trạng thái hội thoại (mute/pin/hidden/archive/unread/autodelete)

| Hàm | Service | Endpoint (Method) | Input | Output | Ghi chú |
|---|---|---|---|---|---|
| SetMute | `profile` | `/api/social/profile/setmute` (POST) | threadID, threadType, {Duration, Action 1 mute/3 unmute} | string | Duration -1=mãi, 0=đến 8h sáng |
| GetMute | `profile` | `/api/social/profile/getmute` (GET) | — | {ChatEntries,GroupChatEntries} | |
| SetPinChat | `conversation` | `/api/pinconvers/updatev2` (POST) | threadID[], threadType, isPinned | string | prefix u/g |
| GetPinnedChat | `conversation` | `/api/pinconvers/list` (GET) | — | {Conversations[]} | |
| SetHiddenChat | `conversation` | `/api/hiddenconvers/add-remove` (POST) | threadID[], threadType, isHidden | string | |
| GetHiddenChat | `conversation` | `/api/hiddenconvers/get-all` (GET) | — | {PIN,Threads[]} | |
| UpdateHiddenChatPIN | `conversation` | `/api/hiddenconvers/update-pin` (GET) | pin (4 số) | string | PIN MD5 hash |
| ResetHiddenChatPIN | `conversation` | `/api/hiddenconvers/reset` (GET) | — | string | |
| UpdateArchivedChatList | `label` | `/api/archivedchat/update` (POST) | isArchived, conversations... | {NeedResync,Version} | |
| GetArchivedChatList | `label` | `/api/archivedchat/list` (GET) | — | {items[],version} | |
| UpdateAutoDeleteChat | `conversation` | `/api/conv/autodelete/updateConvers` (POST) | threadID, threadType, ttl (Off/24H/7D/14D) | string | |
| GetAutoDeleteChat | `conversation` | `/api/conv/autodelete/getConvers` (GET) | — | {converts[]} | |
| AddUnreadMark | `conversation` | `/api/conv/addUnreadMark` (POST) | threadID, threadType | {UpdateID,Status} | |
| RemoveUnreadMark | `conversation` | `/api/conv/removeUnreadMark` (POST) | threadID, threadType | {UpdateID,Status} | |
| GetUnreadMark | `conversation` | `/api/conv/getUnreadMark` (GET) | — | {ConvsUser,ConvsGroup} | |
| DeleteChat | `chat`/`group` | `/api/message/deleteconver`·`/api/group/deleteconver` (POST) | threadID, threadType, {OwnerID,CliMsgID,GlobalMsgID} | {Status} | onlyMe=1 |

## 10. Quick-message / Auto-reply / Label

| Hàm | Service | Endpoint (Method) | Input | Output | Ghi chú |
|---|---|---|---|---|---|
| AddQuickMessage | `quick_message` | `/api/quickmessage/create` (GET) | {Keyword,Title,Media?} | {Item,Version} | có Media → UploadPhoto trước |
| UpdateQuickMessage | `quick_message` | `/api/quickmessage/update` (GET) | itemID, {Keyword,Title,Media?} | {Item,Version} | |
| RemoveQuickMessage | `quick_message` | `/api/quickmessage/delete` (GET) | messageID... | {ItemIDs,Version} | |
| GetQuickMessageList | `quick_message` | `/api/quickmessage/list` (GET) | — | {Items[]} | |
| CreateAutoReply | `auto_reply` | `/api/autoreply/create` (POST) | {Content,IsEnable,StartTime,EndTime,Scope,UIDs[]} | {Item,Version} | |
| UpdateAutoReply | `auto_reply` | `/api/autoreply/update` (POST) | {ID,Content,IsEnable,…} | {Item,Version} | |
| DeleteAutoReply | `auto_reply` | `/api/autoreply/delete` (POST) | id | {Item,Version} | |
| GetAutoReplyList | `auto_reply` | `/api/autoreply/list` (GET) | — | {item[]} | |
| GetLabels | `label` | `/api/convlabel/get` (GET) | — | {LabelData[]} | labelData là string JSON lồng |
| UpdateLabels | `label` | `/api/convlabel/update` (POST) | {LabelData[],Version} | {LabelData[]} | |

---

## Điểm cần lưu ý (gotchas)
- **msgId Zalo trả STRING hoặc NUMBER tùy endpoint** → `SendMessageResult`/`SendBankCardResponse`/`SendLinkResponse` dùng `flexID` (UnmarshalJSON nhận cả hai). Đừng khai `string` cứng.
- **Video BẮT BUỘC** thumbUrl+duration+width+height; **Voice** cần m4a/aac + key `toId` (camelCase).
- **Ảnh/file upload cần Size** (imageMetadataGetter) ⇒ TotalChunk; **video/file async chờ WS `file_done`**.
- **Ký URL SAU khi nối path** cho SendMessage attachment (base includeDefaults=false) — ký sớm ⇒ 404.
- `GetSetting`/`UpdateSetting` dùng **URL hard-code** `wpa.chat.zalo.me`, không GetZpwService.
- Endpoint đã BỎ (404): `getclosedfriends`, `onlines`. Catalog/Product cần OA account.
