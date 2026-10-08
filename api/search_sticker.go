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
	SearchStickerResponse []model.StickerBasic
	SearchStickerFn       = func(ctx context.Context, keyword string, limit int) (*SearchStickerResponse, error)
)

func (a *api) SearchSticker(ctx context.Context, keyword string, limit int) (*SearchStickerResponse, error) {
	return a.e.SearchSticker(ctx, keyword, limit)
}

var searchStickerFactory = apiFactory[*SearchStickerResponse, SearchStickerFn]()(
	func(a *api, sc session.Context, u factoryUtils[*SearchStickerResponse]) (SearchStickerFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("sticker"), "")
		serviceURL := u.MakeURL(base+"/api/message/sticker/search", nil, true)

		return func(ctx context.Context, keyword string, limit int) (*SearchStickerResponse, error) {
			if limit <= 0 {
				limit = 50
			}

			payload := map[string]any{
				"keyword": keyword,
				"limit":   limit,
				"srcType": 0,
				"imei":    sc.IMEI(),
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.SearchSticker", err)
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
