package api

import (
	"context"
	"net/http"

	"github.com/luannt2002/zalo-api-unofficial/errs"
	"github.com/luannt2002/zalo-api-unofficial/internal/httpx"
	"github.com/luannt2002/zalo-api-unofficial/internal/jsonx"
	"github.com/luannt2002/zalo-api-unofficial/model"
	"github.com/luannt2002/zalo-api-unofficial/session"
)

type (
	UpdateAutoReplyRequest struct {
		ID        int                  `json:"id"`
		Content   string               `json:"content"`
		IsEnable  bool                 `json:"isEnable"`
		StartTime int64                `json:"startTime"`
		EndTime   int64                `json:"endTime"`
		Scope     model.AutoReplyScope `json:"scope"`
		UIDs      []string             `json:"uids"`
	}
	UpdateAutoReplyResponse struct {
		Item    model.AutoReplyItem `json:"item"`
		Version int                 `json:"version"`
	}
	UpdateAutoReplyFn = func(ctx context.Context, message UpdateAutoReplyRequest) (*UpdateAutoReplyResponse, error)
)

func (a *api) UpdateAutoReply(ctx context.Context, message UpdateAutoReplyRequest) (*UpdateAutoReplyResponse, error) {
	return a.e.UpdateAutoReply(ctx, message)
}

var updateAutoReplyFactory = apiFactory[*UpdateAutoReplyResponse, UpdateAutoReplyFn]()(
	func(a *api, sc session.Context, u factoryUtils[*UpdateAutoReplyResponse]) (UpdateAutoReplyFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("auto_reply"), "")
		serviceURL := u.MakeURL(base+"/api/autoreply/update", nil, true)

		return func(ctx context.Context, message UpdateAutoReplyRequest) (*UpdateAutoReplyResponse, error) {
			resultUids := make([]string, 0)
			if message.Scope == model.AutoReplyScopeSpecificFriends || message.Scope == model.AutoReplyScopeFriendsExcept {
				resultUids = message.UIDs
			}

			payload := map[string]any{
				"cliLang":    sc.Language(),
				"id":         message.ID,
				"enable":     message.IsEnable,
				"content":    message.Content,
				"startTime":  message.StartTime,
				"endTime":    message.EndTime,
				"recurrence": []string{"RRULE:FREQ=DAILY;"},
				"scope":      message.Scope,
				"uids":       resultUids,
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.UpdateAutoReply", err)
			}

			body := httpx.BuildFormBody(map[string]string{"params": enc})
			resp, err := u.Request(ctx, serviceURL, &httpx.RequestOptions{
				Method: http.MethodPost,
				Body:   body,
			})
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()

			return u.Resolve(resp, true)
		}, nil
	},
)
