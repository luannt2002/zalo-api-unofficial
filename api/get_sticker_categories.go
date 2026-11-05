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
	GetStickerCategoriesResponse struct {
		DataURL   string `json:"dataUrl"`
		CS        string `json:"cs"`
		ExCateIDs []int  `json:"exCateIds"`
		Data      []any  `json:"data"`
	}
	GetStickerCategoriesFn = func(ctx context.Context) (*GetStickerCategoriesResponse, error)
)

func (a *api) GetStickerCategories(ctx context.Context) (*GetStickerCategoriesResponse, error) {
	return a.e.GetStickerCategories(ctx)
}

var getStickerCategoriesFactory = apiFactory[*GetStickerCategoriesResponse, GetStickerCategoriesFn]()(
	func(a *api, sc session.Context, u factoryUtils[*GetStickerCategoriesResponse]) (GetStickerCategoriesFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("sticker"), "")
		serviceURL := u.MakeURL(base+"/api/message/sticker/category/list/v2", nil, true)

		return func(ctx context.Context) (*GetStickerCategoriesResponse, error) {
			payload := map[string]any{}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.GetStickerCategories", err)
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
