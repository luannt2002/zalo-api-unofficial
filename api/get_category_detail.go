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
	GetCategoryDetailResponse = model.CategoryDetail
	GetCategoryDetailFn       = func(ctx context.Context, cateID int) (*GetCategoryDetailResponse, error)
)

func (a *api) GetCategoryDetail(ctx context.Context, cateID int) (*GetCategoryDetailResponse, error) {
	return a.e.GetCategoryDetail(ctx, cateID)
}

var getCategoryDetailFactory = apiFactory[*GetCategoryDetailResponse, GetCategoryDetailFn]()(
	func(a *api, sc session.Context, u factoryUtils[*GetCategoryDetailResponse]) (GetCategoryDetailFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("sticker"), "")
		serviceURL := u.MakeURL(base+"/api/message/sticker/category/detail", nil, true)

		return func(ctx context.Context, cateID int) (*GetCategoryDetailResponse, error) {
			payload := map[string]any{
				"cid": cateID,
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.GetCategoryDetail", err)
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
