package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/luannt2002/zalo-api-unofficial/errs"
	"github.com/luannt2002/zalo-api-unofficial/internal/httpx"
	"github.com/luannt2002/zalo-api-unofficial/internal/jsonx"
	"github.com/luannt2002/zalo-api-unofficial/session"
)

type (
	RenewMsgInfo struct {
		PhotoID    string `json:"photoId,omitempty"`
		Title      string `json:"title,omitempty"`
		OriURL     string `json:"oriUrl,omitempty"`
		URL        string `json:"url,omitempty"`
		ThumbURL   string `json:"thumbUrl,omitempty"`
		HDURL      string `json:"hdUrl,omitempty"`
		NormalURL  string `json:"normalUrl,omitempty"`
		Width      int    `json:"width,omitempty"`
		Height     int    `json:"height,omitempty"`
		IsE2EE     int    `json:"isE2EE"`
		IsOriginal int    `json:"isOriginal"`
	}

	RenewLinkRequest struct {
		ToID     string       `json:"toId"`
		MsgType  int          `json:"msgType"` // 1: File, 2: Photo (mặc định), 5: Video
		MsgInfo  RenewMsgInfo `json:"msgInfo"`
		ClientID string       `json:"clientId,omitempty"`
		IsGroup  *int         `json:"isGroup,omitempty"` // 1 nếu toId là nhóm hoặc Cloud của tôi (send2me)
	}

	RenewLinkResponse struct {
		Async       bool   `json:"async"`
		FileURL     string `json:"fileUrl,omitempty"`
		Normal      string `json:"normal,omitempty"`
		HD          string `json:"hd,omitempty"`
		Thumb       string `json:"thumb,omitempty"`
		ExpiredTime int64  `json:"expiredTime,omitempty"`
	}

	RenewLinkFn = func(ctx context.Context, req RenewLinkRequest) (*RenewLinkResponse, error)
)

func (a *api) RenewLink(ctx context.Context, req RenewLinkRequest) (*RenewLinkResponse, error) {
	return a.e.RenewLink(ctx, req)
}

var renewLinkFactory = apiFactory[*RenewLinkResponse, RenewLinkFn]()(
	func(a *api, sc session.Context, u factoryUtils[*RenewLinkResponse]) (RenewLinkFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("file"), "")
		serviceURL := u.MakeURL(base+"/api/message/renewlink", nil, true)

		return func(ctx context.Context, req RenewLinkRequest) (*RenewLinkResponse, error) {
			if req.ToID == "" {
				req.ToID = sc.Send2meID()
			}
			if req.ToID == "" {
				return nil, errs.NewZCA("toId cannot be empty", "api.RenewLink")
			}

			// Mặc định msgType là 2 (Photo) nếu chưa truyền
			msgType := req.MsgType
			if msgType == 0 {
				msgType = 2
			}

			// Chuẩn hoá msgInfo: Zalo server bắt buộc có oriUrl, thumbUrl, width, height
			msgInfo := req.MsgInfo
			if msgInfo.OriURL == "" {
				if msgInfo.NormalURL != "" {
					msgInfo.OriURL = msgInfo.NormalURL
				} else if msgInfo.URL != "" {
					msgInfo.OriURL = msgInfo.URL
				}
			}
			if msgInfo.NormalURL == "" && msgInfo.OriURL != "" {
				msgInfo.NormalURL = msgInfo.OriURL
			}
			if msgInfo.ThumbURL == "" && msgInfo.OriURL != "" {
				msgInfo.ThumbURL = msgInfo.OriURL
			}
			if msgInfo.HDURL == "" && msgInfo.OriURL != "" {
				msgInfo.HDURL = msgInfo.OriURL
			}
			if msgInfo.Width <= 0 {
				msgInfo.Width = 1
			}
			if msgInfo.Height <= 0 {
				msgInfo.Height = 1
			}

			// Quy tắc bắt buộc bóc từ Web bundle (1.13ceb43a998bc76de243.js):
			// isGroup = 1 nếu toId bắt đầu bằng "g_" hoặc toId == send2me_id (Cloud của tôi)
			isGroup := 0
			if req.IsGroup != nil {
				isGroup = *req.IsGroup
			} else if sc.IsSend2me(req.ToID) || strings.HasPrefix(req.ToID, "g_") || strings.HasPrefix(req.ToID, "g") {
				isGroup = 1
			}

			clientId := req.ClientID
			if clientId == "" {
				clientId = fmt.Sprint(time.Now().UnixMilli())
			}

			body := map[string]any{
				"toId":     req.ToID,
				"msgType":  msgType,
				"msgInfo":  jsonx.Stringify(msgInfo),
				"clientId": clientId,
				"imei":     sc.IMEI(),
				"isGroup":  isGroup,
			}

			enc, err := u.EncodeAES(jsonx.Stringify(body))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt renew params", "api.RenewLink", err)
			}

			url := u.MakeURL(serviceURL, map[string]any{"params": enc}, true)
			resp, err := u.Request(ctx, url, &httpx.RequestOptions{Method: http.MethodPost})
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()

			return u.Resolve(resp, true)
		}, nil
	},
)
