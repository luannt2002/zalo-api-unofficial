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
	GetStickerCategoryDetailResponse []model.StickerDetail
	GetStickerCategoryDetailFn       = func(ctx context.Context, cateID int) (*GetStickerCategoryDetailResponse, error)
)

func (a *api) GetStickerCategoryDetail(ctx context.Context, cateID int) (*GetStickerCategoryDetailResponse, error) {
	return a.e.GetStickerCategoryDetail(ctx, cateID)
}

var getStickerCategoryDetailFactory = apiFactory[*GetStickerCategoryDetailResponse, GetStickerCategoryDetailFn]()(
	func(a *api, sc session.Context, u factoryUtils[*GetStickerCategoryDetailResponse]) (GetStickerCategoryDetailFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("sticker"), "")
		serviceURL := u.MakeURL(base+"/api/message/sticker/category/sticker_detail", nil, true)

		return func(ctx context.Context, cateID int) (*GetStickerCategoryDetailResponse, error) {
			payload := map[string]any{
				"cid": cateID,
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.GetStickerCategoryDetail", err)
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
