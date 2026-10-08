package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image/gif"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	"github.com/luannt2002/zalo-api-unofficial/config"
	"github.com/luannt2002/zalo-api-unofficial/errs"
	"github.com/luannt2002/zalo-api-unofficial/internal/httpx"
	"github.com/luannt2002/zalo-api-unofficial/internal/jsonx"
	"github.com/luannt2002/zalo-api-unofficial/model"
	"github.com/luannt2002/zalo-api-unofficial/session"
)

var (
	ErrMessageContentEmpty        = errs.NewZCA("message content cannot be empty", "api.SendMessage")
	ErrInvalidMention             = errs.NewZCA("Invalid mentions: total mention characters exceed message length", "api.SendMessage")
	ErrInvalidWebchatQuote        = errs.NewZCA("invalid quote: content must be string for msgType 'webchat'", "api.SendMessage")
	ErrUnsupportedQuotedGroupPoll = errs.NewZCA("quoted message type 'group.poll' is not supported", "api.SendMessage")
)

type TextStyle string

const (
	TextStyleBold          TextStyle = "b"
	TextStyleItalic        TextStyle = "i"
	TextStyleUnderline     TextStyle = "u"
	TextStyleStrikeThrough TextStyle = "s"

	TextStyleRed    TextStyle = "c_db342e"
	TextStyleOrange TextStyle = "c_f27806"
	TextStyleYellow TextStyle = "c_f7b503"
	TextStyleGreen  TextStyle = "c_15a85f"

	TextStyleSmall TextStyle = "f_13"
	TextStyleBig   TextStyle = "f_18"

	TextStyleOrderedList   TextStyle = "lst_2"
	TextStyleUnorderedList TextStyle = "lst_1"

	TextStyleIndent TextStyle = "ind_$"
)

type (
	SendMessageQuote struct {
		MsgID    string `json:"msgId"`
		CliMsgID string `json:"cliMsgId"`
		MsgType  string `json:"msgType"`
		UIDFrom  string `json:"uidFrom"`

		Content     model.Content      `json:"content"`
		PropertyExt *model.PropertyExt `json:"propertyExt,omitempty"`

		TS  string `json:"ts"`
		TTL int    `json:"ttl"`
	}
	MessageStyle struct {
		Start int       `json:"start"`
		Len   int       `json:"len"`
		Style TextStyle `json:"st"`

		IndentSize int `json:"indentSize"` // Used for indent style
	}
	MessageContent struct {
		Msg     string
		Style   []MessageStyle
		Urgency model.Urgency

		Quote       *SendMessageQuote
		Mentions    []model.TMention
		Attachments []model.AttachmentSource
		TTL         int // Time to live in milliseconds
	}

	sendData struct {
		URL     string
		Body    io.Reader
		Headers http.Header
	}

	SendMessageResult struct {
		MsgID string `json:"msgId"`
	}
	SendMessageResponse struct {
		Message    *SendMessageResult  `json:"message"`
		Attachment []SendMessageResult `json:"attachment"`
	}
	SendMessageFn = func(ctx context.Context, threadID string, threadType model.ThreadType, message MessageContent) (*SendMessageResponse, error)
)

// flexID nhận msgId của Zalo DÙ trả STRING ("123") hay NUMBER (123) — quy về string.
// Zalo KHÔNG nhất quán kiểu msgId giữa các endpoint (ảnh/text=string, file/card=number).
// Dùng cho SendBankCard/SendLink... để tránh "cannot unmarshal number/object into string".
type flexID string

func (f *flexID) UnmarshalJSON(b []byte) error {
	s := strings.Trim(strings.TrimSpace(string(b)), `"`)
	if s == "null" {
		s = ""
	}
	*f = flexID(s)
	return nil
}

// UnmarshalJSON cho msgId DÙ Zalo trả STRING ("123") hay NUMBER (123). Ảnh/text
// (photo_original/send, /api/message) trả msgId dạng CHUỖI; file (asyncfile/msg) trả
// dạng SỐ. Trước đây khai `MsgID string` ⇒ file lỗi "cannot unmarshal number into ...
// msgId of type string" ⇒ MẤT msg_id thật (lưu local:..., media bền + tracking hỏng).
func (r *SendMessageResult) UnmarshalJSON(b []byte) error {
	var raw struct {
		MsgID json.RawMessage `json:"msgId"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	r.MsgID = strings.Trim(string(raw.MsgID), `"`)
	if r.MsgID == "null" {
		r.MsgID = ""
	}
	return nil
}

func (a *api) SendMessage(ctx context.Context, threadID string, threadType model.ThreadType, message MessageContent) (*SendMessageResponse, error) {
	return a.e.SendMessage(ctx, threadID, threadType, message)
}

var sendMessageFactory = apiFactory[*SendMessageResponse, SendMessageFn]()(
	func(a *api, sc session.Context, u factoryUtils[*SendMessageResponse]) (SendMessageFn, error) {
		fileBase := jsonx.FirstOr(sc.GetZpwService("file"), "")
		userBase := jsonx.FirstOr(sc.GetZpwService("chat"), "")
		groupBase := jsonx.FirstOr(sc.GetZpwService("group"), "")
		defaultParams := map[string]any{"nretry": 0}
		shareFile := sc.Settings().Features.ShareFile

		serviceURLs := struct {
			Message    map[model.ThreadType]string
			Attachment map[model.ThreadType]string
		}{
			Message: map[model.ThreadType]string{
				model.ThreadTypeUser:  u.MakeURL(userBase+"/api/message", defaultParams, true),
				model.ThreadTypeGroup: u.MakeURL(groupBase+"/api/group", defaultParams, true),
			},
			// includeDefaults=FALSE: đây là URL BASE, sẽ được nối thêm path
			// ("/photo_original/send", "/gif", "/asyncfile/msg") RỒI MỚI MakeURL(...,true)
			// để ký. Nếu ký ở đây (true) thì base đã có "?zpw_ver=..."; nối path vào sau
			// query ⇒ path kẹt trong query ⇒ Zalo 404. (Upload ở upload_attachment.go
			// cũng build base trần rồi mới ký — cùng lý do.)
			Attachment: map[model.ThreadType]string{
				model.ThreadTypeUser:  u.MakeURL(fileBase+"/api/message", nil, false),
				model.ThreadTypeGroup: u.MakeURL(fileBase+"/api/group", nil, false),
			},
		}

		isExceedMaxFile := func(totalFile int) bool {
			return totalFile > shareFile.MaxFile
		}

		isExceedMaxFileSize := func(fileSize int64) bool {
			return fileSize > shareFile.MaxSizeShareFileV3*1024*1024
		}

		handleStyles := func(payload map[string]any, mStyles []MessageStyle) {
			if len(mStyles) == 0 {
				return
			}

			styles := make([]map[string]any, 0, len(mStyles))
			for _, s := range mStyles {
				st := s.Style
				if st == TextStyleIndent {
					size := s.IndentSize
					if size <= 0 {
						size = 1
					}
					repl := fmt.Sprintf("%d0", size)
					st = TextStyle(strings.ReplaceAll(string(TextStyleIndent), "$", repl))
				}

				styles = append(styles, map[string]any{
					"start": s.Start,
					"len":   s.Len,
					"st":    st,
				})
			}

			payload["textProperties"] = jsonx.Stringify(map[string]any{
				"styles": styles,
				"ver":    0,
			})
		}

		handleQuoteMessage := func(payload map[string]any, quote *SendMessageQuote, isGroup bool) {
			payload["qmsgOwner"] = quote.UIDFrom
			payload["qmsgId"] = quote.MsgID
			payload["qmsgCliId"] = quote.CliMsgID
			payload["qmsgType"] = quote.GetMessageType()
			payload["qmsgTs"] = quote.TS
			payload["qmsgTTL"] = quote.TTL

			if quote.Content.String != nil {
				payload["qmsg"] = quote.Content.String
			} else {
				payload["qmsg"] = quote.BuildMessagePayload()
			}

			if isGroup {
				payload["qmsgAttach"] = jsonx.Stringify(quote.BuildAttachmentMessagePayload())
			}
		}

		handleMessage := func(threadID string, threadType model.ThreadType, message MessageContent) (*sendData, error) {
			quote := message.Quote
			isGroup := threadType == model.ThreadTypeGroup
			if message.Quote != nil {
				if quote.Content.String == nil && quote.MsgType == "webchat" {
					return nil, ErrInvalidWebchatQuote
				}

				if quote.MsgType == "group.poll" {
					return nil, ErrUnsupportedQuotedGroupPoll
				}
			}

			payload := map[string]any{
				"message":  message.Msg,
				"clientId": time.Now().UnixMilli(),
				"ttl":      message.TTL,
			}

			if isGroup {
				payload["grid"] = threadID
				payload["mentionInfo"] = jsonx.Stringify(message.Mentions)
				payload["visibility"] = 0
			} else {
				payload["toid"] = threadID
				payload["imei"] = sc.IMEI()
			}

			path := "/sendmsg"
			if quote != nil {
				handleQuoteMessage(payload, quote, isGroup)
				path = "/quote"
			} else if !isGroup {
				path = "/sms"
			} else if payload["mentionInfo"] != nil {
				path = "/mention"
			}

			if message.Urgency != model.UrgDefault {
				payload["metaData"] = map[string]any{"urgency": message.Urgency}
			}

			handleStyles(payload, message.Style)

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.SendMessage", err)
			}

			url, err := url.Parse(serviceURLs.Message[threadType])
			if err != nil {
				return nil, errs.WrapZCA("failed to parse message URL", "api.SendMessage", err)
			}

			url.Path += path
			body := httpx.BuildFormBody(map[string]string{"params": enc})

			return &sendData{
				URL:  url.String(),
				Body: body,
			}, nil
		}

		handleUrgency := func(payload map[string]any, urgency model.Urgency) {
			if urgency == model.UrgImportant || urgency == model.UrgUrgent {
				payload["metaData"] = map[string]any{"urgency": urgency}
			}
		}

		// handleGIFAttachment uploads a thumbnail then builds the chunked gif send
		// request(s). It mirrors the dedicated SendGIF flow but inlines the message
		// text/urgency so a gif can travel alongside other attachments.
		handleGIFAttachment := func(
			ctx context.Context,
			threadID string,
			threadType model.ThreadType,
			source model.AttachmentSource,
			message MessageContent,
			exceedMaxSize func(int64) bool,
		) ([]sendData, error) {
			var (
				reader   io.Reader
				closer   io.Closer
				fileName string
				meta     model.AttachmentMetadata
			)

			if f := source.String(); f != "" {
				r, err := os.Open(f)
				if err != nil {
					return nil, errs.WrapZCA("failed to read file", "api.SendMessage", err)
				}
				reader = r
				closer = r

				meta, fileName, err = sc.GetImageMetadata(f)
				if err != nil {
					_ = r.Close()
					return nil, err
				}
			} else if f := source.Object(); f != nil {
				reader, fileName, meta = f.Data, f.Filename, f.Metadata
			} else {
				return nil, errs.ErrSourceEmpty
			}

			rawData, err := io.ReadAll(reader)
			if closer != nil {
				_ = closer.Close()
			}
			if err != nil {
				return nil, errs.WrapZCA("failed to read attachment data", "api.SendMessage", err)
			}

			if exceedMaxSize(meta.Size) {
				return nil, errs.ErrExceedMaxFileSize
			}

			// Build a PNG thumbnail from the first gif frame.
			g, err := gif.DecodeAll(bytes.NewReader(rawData))
			if err != nil {
				return nil, errs.WrapZCA("failed to decode gif for thumbnail", "api.SendMessage", err)
			}
			first := g.Image[0]
			b := first.Bounds()

			var thumbBuf bytes.Buffer
			if err := png.Encode(&thumbBuf, first); err != nil {
				return nil, errs.WrapZCA("failed to encode png thumbnail", "api.SendMessage", err)
			}

			thumbSource, err := model.NewObjectAttachment(fileName, model.AttachmentMetadata{
				Size:   int64(thumbBuf.Len()),
				Width:  b.Dx(),
				Height: b.Dy(),
			}, bytes.NewReader(thumbBuf.Bytes()))
			if err != nil {
				return nil, err
			}

			thumb, err := a.UploadThumbnail(ctx, *thumbSource)
			if err != nil {
				return nil, errs.WrapZCA("failed to upload thumbnail", "api.SendMessage", err)
			}

			forms, err := httpx.BuildFormData(
				"chunkContent", bytes.NewReader(rawData),
				httpx.WithContentType("application/octet-stream"),
				httpx.WithFileName(fileName),
				httpx.WithChunkSize(config.GIFChunkSize),
			)
			if err != nil || len(forms) == 0 || forms[0] == nil {
				return nil, errs.WrapZCA("failed to build form data", "api.SendMessage", err)
			}

			checksum := ""
			if md5 := source.GetLargeFileMD5(); md5 != nil {
				checksum = md5.Checksum
			}

			gifBase := serviceURLs.Attachment[threadType] + "/gif"
			out := make([]sendData, 0, len(forms))
			for i := range forms {
				payload := map[string]any{
					"clientId":   strconv.FormatInt(time.Now().UnixMilli(), 10),
					"fileName":   fileName,
					"totalSize":  meta.Size,
					"width":      meta.Width,
					"height":     meta.Height,
					"msg":        message.Msg,
					"type":       1,
					"ttl":        message.TTL,
					"thumb":      thumb.URL,
					"checksum":   checksum,
					"totalChunk": len(forms),
					"chunkId":    i + 1,
				}

				if threadType == model.ThreadTypeGroup {
					payload["visibility"] = 0
					payload["grid"] = threadID
				} else {
					payload["toid"] = threadID
				}

				handleUrgency(payload, message.Urgency)

				enc, err := u.EncodeAES(jsonx.Stringify(payload))
				if err != nil {
					return nil, errs.WrapZCA("failed to encrypt params", "api.SendMessage", err)
				}

				url := u.MakeURL(gifBase, map[string]any{"type": "1", "params": enc, "nretry": "0"}, true)
				out = append(out, sendData{
					URL:     url,
					Body:    forms[i].Body,
					Headers: forms[i].Header,
				})
			}

			return out, nil
		}

		// handleMentions filters mentions valid for the thread (group-only) and
		// validates that the total mention length does not exceed the message length.
		handleMentions := func(threadType model.ThreadType, msg string, mentions []model.TMention) ([]model.TMention, error) {
			if threadType != model.ThreadTypeGroup || len(mentions) == 0 {
				return nil, nil
			}

			final := make([]model.TMention, 0, len(mentions))
			totalLen := 0
			for _, m := range mentions {
				if m.Pos < 0 || m.UID == "" || m.Len <= 0 {
					continue
				}
				totalLen += m.Len
				mt := model.MentionEach
				if m.UID == model.MentionAllUID {
					mt = model.MentionType(1)
				}
				final = append(final, model.TMention{
					Pos:  m.Pos,
					UID:  m.UID,
					Len:  m.Len,
					Type: mt,
				})
			}

			if totalLen > len(msg) {
				return nil, ErrInvalidMention
			}
			return final, nil
		}

		handleAttachment := func(ctx context.Context, threadID string, threadType model.ThreadType, message MessageContent) ([]sendData, error) {
			if len(message.Attachments) == 0 {
				return nil, errs.ErrSourceEmpty
			}

			isGroup := threadType == model.ThreadTypeGroup

			// Separate gif files from regular attachments; gif uses a dedicated flow.
			gifFiles := make([]model.AttachmentSource, 0)
			uploadSources := make([]model.AttachmentSource, 0, len(message.Attachments))
			for _, src := range message.Attachments {
				if src.GetExtension() == string(model.FileTypeGif) {
					gifFiles = append(gifFiles, src)
				} else {
					uploadSources = append(uploadSources, src)
				}
			}

			var uploaded UploadAttachmentResponse
			if len(uploadSources) > 0 {
				up, err := a.UploadAttachment(ctx, threadID, threadType, uploadSources...)
				if err != nil {
					return nil, err
				}
				uploaded = up
			}

			mentionsFinal, err := handleMentions(threadType, message.Msg, message.Mentions)
			if err != nil {
				return nil, err
			}

			// Mentions can only be attached as a photo description when there is a
			// single image attachment in a group and no quote.
			canBeDesc := message.IsPhotoDescription()
			isMentionsValid := len(mentionsFinal) > 0 && isGroup && len(uploadSources) == 1

			isMultiFile := len(uploadSources) > 1
			groupLayoutID := strconv.FormatInt(time.Now().UnixMilli(), 10)

			type attachmentEntry struct {
				fileType model.FileType
				params   map[string]any
			}

			entries := make([]attachmentEntry, 0, len(uploaded)+len(gifFiles))

			clientID := time.Now().UnixMilli()
			indexInGroup := 0

			for _, att := range uploaded {
				params := map[string]any{
					"clientId": strconv.FormatInt(clientID, 10),
					"zsource":  -1,
					"ttl":      message.TTL,
				}
				clientID++

				if isGroup {
					params["grid"] = threadID
				} else {
					params["toid"] = threadID
				}

				switch att.FileType {
				case model.FileTypeImage:
					img := att.Image
					if img == nil {
						return nil, errs.NewZCA("missing image upload info", "api.SendMessage")
					}

					params["photoId"] = img.PhotoID
					params["desc"] = message.Msg
					params["width"] = img.Width
					params["height"] = img.Height
					params["rawUrl"] = img.NormalURL
					params["hdUrl"] = img.HDURL
					params["thumbUrl"] = img.ThumbURL
					params["hdSize"] = strconv.FormatInt(img.HDSize, 10)
					params["jcp"] = `{"convertible":"jxl"}`

					if isGroup {
						params["oriUrl"] = img.NormalURL
					} else {
						params["normalUrl"] = img.NormalURL
					}

					if isMultiFile {
						params["groupLayoutId"] = groupLayoutID
						params["isGroupLayout"] = 1
						params["idInGroup"] = indexInGroup
						params["totalItemInGroup"] = len(uploaded)
						params["extMsgProp"] = fmt.Sprintf(`{"groupMediaMsg":{"groupLayoutId":"%s"}}`, groupLayoutID)
						indexInGroup++
					}

					if isMentionsValid && canBeDesc && message.Quote == nil {
						params["mentionInfo"] = jsonx.Stringify(mentionsFinal)
					}

				case model.FileTypeVideo, model.FileTypeOther:
					file := att.File
					if file == nil {
						return nil, errs.NewZCA("missing file upload info", "api.SendMessage")
					}

					ext := ""
					if idx := strings.LastIndex(file.FileName, "."); idx >= 0 && idx+1 < len(file.FileName) {
						ext = strings.ToLower(file.FileName[idx+1:])
					}

					params["fileId"] = file.FileID
					params["checksum"] = file.Checksum
					params["checksumSha"] = ""
					params["extention"] = ext
					params["totalSize"] = att.TotalSize
					params["fileName"] = file.FileName
					params["clientId"] = strconv.Itoa(att.ClientFileID)
					params["fType"] = 1
					params["fileCount"] = 0
					params["fdata"] = "{}"
					params["fileUrl"] = file.FileURL
				}

				handleUrgency(params, message.Urgency)
				entries = append(entries, attachmentEntry{fileType: att.FileType, params: params})
			}

			data := make([]sendData, 0, len(entries)+len(gifFiles))

			attachmentBase := serviceURLs.Attachment[threadType]
			attachmentURLType := map[model.FileType]string{
				model.FileTypeImage: "/photo_original/send",
				model.FileTypeVideo: "/asyncfile/msg",
				model.FileTypeOther: "/asyncfile/msg",
			}

			for _, entry := range entries {
				enc, err := u.EncodeAES(jsonx.Stringify(entry.params))
				if err != nil {
					return nil, errs.WrapZCA("failed to encrypt params", "api.SendMessage", err)
				}

				url := u.MakeURL(
					attachmentBase+attachmentURLType[entry.fileType],
					map[string]any{"nretry": "0"},
					true,
				)

				data = append(data, sendData{
					URL:  url,
					Body: httpx.BuildFormBody(map[string]string{"params": enc}),
				})
			}

			// GIF attachments: upload thumbnail then send the gif chunk(s).
			for _, gifSrc := range gifFiles {
				gifData, err := handleGIFAttachment(ctx, threadID, threadType, gifSrc, message, isExceedMaxFileSize)
				if err != nil {
					return nil, err
				}
				data = append(data, gifData...)
			}

			return data, nil
		}

		sendMessage := func(ctx context.Context, sendData []sendData) ([]SendMessageResult, error) {
			var (
				mu      sync.Mutex
				g, gctx = errgroup.WithContext(ctx)
				results = make([]SendMessageResult, 0, len(sendData))
			)

			for _, data := range sendData {
				g.Go(func() error {
					resp, err := u.Request(gctx, data.URL, &httpx.RequestOptions{
						Method:  http.MethodPost,
						Body:    data.Body,
						Headers: data.Headers,
					})
					if err != nil {
						return err
					}
					defer resp.Body.Close()

					res, err := resolveResponse[SendMessageResult](sc, resp, true)
					if err != nil {
						return err
					}

					mu.Lock()
					results = append(results, res)
					mu.Unlock()

					return nil
				})
			}

			if err := g.Wait(); err != nil {
				return nil, err
			}
			return results, nil
		}

		return func(ctx context.Context, threadID string, threadType model.ThreadType, message MessageContent) (*SendMessageResponse, error) {
			if len(message.Msg) == 0 && (len(message.Attachments) == 0) {
				return nil, ErrMessageContentEmpty
			}
			if isExceedMaxFile(len(message.Attachments)) {
				return nil, errs.ErrExceedMaxFile
			}

			hasText := func() bool { return len(message.Msg) > 0 }
			hasAttachments := func() bool { return len(message.Attachments) > 0 }

			results := &SendMessageResponse{}

			sendText := func() error {
				data, err := handleMessage(threadID, threadType, message)
				if err != nil {
					return err
				}
				resps, err := sendMessage(ctx, []sendData{*data})
				if err != nil {
					return err
				}
				if len(resps) > 0 {
					results.Message = &resps[0]
				}
				return nil
			}

			sendAttachments := func() error {
				data, err := handleAttachment(ctx, threadID, threadType, message)
				if err != nil {
					return err
				}
				resps, err := sendMessage(ctx, data)
				if err != nil {
					return err
				}
				results.Attachment = resps
				return nil
			}

			if hasAttachments() && hasText() && (!message.IsPhotoDescription() || message.Quote != nil) {
				if err := sendText(); err != nil {
					return nil, err
				}
				message.Msg = ""
				message.Mentions = nil
			}
			if hasAttachments() {
				if err := sendAttachments(); err != nil {
					return nil, err
				}
				message.Msg = ""
			}
			if hasText() {
				if err := sendText(); err != nil {
					return nil, err
				}
			}

			return results, nil
		}, nil
	},
)

func (q *SendMessageQuote) BuildAttachmentMessagePayload() any {
	if q.Content.String != nil {
		return q.PropertyExt
	}

	if q.MsgType == "chat.todo" {
		return map[string]any{
			"properties": model.PropertyExt{
				Color:   0,
				Size:    0,
				Type:    0,
				SubType: 0,
				Ext:     `{"shouldParseLinkOrContact":0}`,
			},
		}
	}

	a := q.Content.Attachment
	return map[string]any{
		"title":       a.Title,
		"description": a.Description,
		"href":        a.Href,
		"thumbUrl":    a.Thumb,
		"oriUrl":      a.Href,
		"normalUrl":   a.Href,
		"childnumber": a.ChildNumber,
		"action":      a.Action,
		"params":      a.Params,
		"type":        a.Type,
	}
}

func (q *SendMessageQuote) BuildMessagePayload() any {
	if q.MsgType != "chat.todo" {
		return ""
	}

	hasRef := (q.Content.Attachment != nil) || (q.Content.Other != nil)
	if !hasRef {
		return ""
	}

	s := ""
	if q.Content.Attachment != nil {
		s = q.Content.Attachment.Params
	} else if q.Content.Other != nil {
		if val, ok := q.Content.Other["params"].(string); ok {
			s = val
		}
	}

	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	var payload struct {
		Item struct {
			Content any `json:"content"`
		} `json:"item"`
	}
	if err := json.Unmarshal([]byte(s), &payload); err != nil {
		return ""
	}
	return payload.Item.Content
}

func (q *SendMessageQuote) GetMessageType() int {
	switch q.MsgType {
	case "webchat":
		return 1
	case "chat.voice":
		return 31
	case "chat.photo":
		return 32
	case "chat.sticker":
		return 36
	case "chat.doodle":
		return 37
	case "chat.recommended":
		return 38
	case "chat.link":
		return 38 // don't know || if (msgType === "chat.link") return 1;
	case "chat.video.msg":
		return 44 // not sure
	case "share.file":
		return 46
	case "chat.gif":
		return 49
	case "chat.location.new":
		return 43
	default:
		return 1
	}
}

func (c *MessageContent) IsPhotoDescription() bool {
	return len(c.Attachments) == 1 && slices.Contains(config.SupportedImageExtensions, c.Attachments[0].GetExtension())
}
