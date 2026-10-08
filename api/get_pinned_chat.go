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
	GetPinnedChatResponse struct {
		Conversations []string `json:"conversations"`
		Version       int      `json:"version"`
	}
	GetPinnedChatFn = func(ctx context.Context) (*GetPinnedChatResponse, error)
)

func (a *api) GetPinnedChat(ctx context.Context) (*GetPinnedChatResponse, error) {
	return a.e.GetPinnedChat(ctx)
}

var getPinnedChatFactory = apiFactory[*GetPinnedChatResponse, GetPinnedChatFn]()(
	func(a *api, sc session.Context, u factoryUtils[*GetPinnedChatResponse]) (GetPinnedChatFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("conversation"), "")
		serviceURL := u.MakeURL(base+"/api/pinconvers/list", nil, true)

		return func(ctx context.Context) (*GetPinnedChatResponse, error) {
			payload := map[string]any{
				"imei": sc.IMEI(),
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.GetPinnedChat", err)
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
