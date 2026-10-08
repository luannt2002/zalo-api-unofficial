package api

import (
	"context"
	"net/http"
	"time"

	"github.com/luannt2002/zalo-api-unofficial/errs"
	"github.com/luannt2002/zalo-api-unofficial/internal/httpx"
	"github.com/luannt2002/zalo-api-unofficial/internal/jsonx"
	"github.com/luannt2002/zalo-api-unofficial/model"
	"github.com/luannt2002/zalo-api-unofficial/session"
)

type (
	UpdateArchivedChatListTarget struct {
		ID   string           `json:"id"`
		Type model.ThreadType `json:"type"`
	}

	UpdateArchivedChatListResponse struct {
		NeedResync bool  `json:"needResync"`
		Version    int64 `json:"version"`
	}
	UpdateArchivedChatListFn = func(ctx context.Context, isArchived bool, conversations ...UpdateArchivedChatListTarget) (*UpdateArchivedChatListResponse, error)
)

func (a *api) UpdateArchivedChatList(ctx context.Context, isArchived bool, conversations ...UpdateArchivedChatListTarget) (*UpdateArchivedChatListResponse, error) {
	return a.e.UpdateArchivedChatList(ctx, isArchived, conversations...)
}

var updateArchivedChatListFactory = apiFactory[*UpdateArchivedChatListResponse, UpdateArchivedChatListFn]()(
	func(a *api, sc session.Context, u factoryUtils[*UpdateArchivedChatListResponse]) (UpdateArchivedChatListFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("label"), "")
		serviceURL := u.MakeURL(base+"/api/archivedchat/update", nil, true)

		return func(ctx context.Context, isArchived bool, conversations ...UpdateArchivedChatListTarget) (*UpdateArchivedChatListResponse, error) {
			ids := make([]any, len(conversations))
			for i, c := range conversations {
				ids[i] = map[string]any{
					"id":   c.ID,
					"type": c.Type,
				}
			}

			payload := map[string]any{
				"actionType": jsonx.B2I(!isArchived),
				"ids":        ids,
				"imei":       sc.IMEI(),
				"version":    time.Now().UnixMilli(),
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.UpdateArchivedChatList", err)
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
