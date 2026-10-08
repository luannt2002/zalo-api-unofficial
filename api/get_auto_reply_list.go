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
	GetAutoReplyListResponse struct {
		Item    []model.AutoReplyItem `json:"item"`
		Version int                   `json:"version"`
	}
	GetAutoReplyListFn = func(ctx context.Context) (*GetAutoReplyListResponse, error)
)

func (a *api) GetAutoReplyList(ctx context.Context) (*GetAutoReplyListResponse, error) {
	return a.e.GetAutoReplyList(ctx)
}

var getAutoReplyListFactory = apiFactory[*GetAutoReplyListResponse, GetAutoReplyListFn]()(
	func(a *api, sc session.Context, u factoryUtils[*GetAutoReplyListResponse]) (GetAutoReplyListFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("auto_reply"), "")
		serviceURL := u.MakeURL(base+"/api/autoreply/list", nil, true)

		return func(ctx context.Context) (*GetAutoReplyListResponse, error) {
			payload := map[string]any{
				"version": 0,
				"cliLang": sc.Language(),
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.GetAutoReplyList", err)
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
