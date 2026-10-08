package api

import (
	"context"
	"net/http"

	"github.com/luannt2002/zalo-api-unofficial/errs"
	"github.com/luannt2002/zalo-api-unofficial/internal/httpx"
	"github.com/luannt2002/zalo-api-unofficial/internal/jsonx"
	"github.com/luannt2002/zalo-api-unofficial/session"
)

type (
	GetArchivedChatListResponse struct {
		Items   []any `json:"items"`
		Version int64 `json:"version"`
	}
	GetArchivedChatListFn = func(ctx context.Context) (*GetArchivedChatListResponse, error)
)

func (a *api) GetArchivedChatList(ctx context.Context) (*GetArchivedChatListResponse, error) {
	return a.e.GetArchivedChatList(ctx)
}

var getArchivedChatListFactory = apiFactory[*GetArchivedChatListResponse, GetArchivedChatListFn]()(
	func(a *api, sc session.Context, u factoryUtils[*GetArchivedChatListResponse]) (GetArchivedChatListFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("label"), "")
		serviceURL := u.MakeURL(base+"/api/archivedchat/list", nil, true)

		return func(ctx context.Context) (*GetArchivedChatListResponse, error) {
			payload := map[string]any{
				"version": 1,
				"imei":    sc.IMEI(),
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.GetArchivedChatList", err)
			}

			url := u.MakeURL(serviceURL, map[string]any{"params": enc}, true)
			resp, err := u.Request(ctx, url, &httpx.RequestOptions{Method: http.MethodGet})
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()

			return u.Resolve(resp, true)
		}, nil
	},
)
