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
	GetTenorStickerMappingResponse struct {
		TenorStickerMap map[string]model.TenorSticker `json:"tenor_sticker_map"`
		ExpiredTime     int64                         `json:"expired_time"`
	}
	GetTenorStickerMappingFn = func(ctx context.Context) (*GetTenorStickerMappingResponse, error)
)

func (a *api) GetTenorStickerMapping(ctx context.Context) (*GetTenorStickerMappingResponse, error) {
	return a.e.GetTenorStickerMapping(ctx)
}

var getTenorStickerMappingFactory = apiFactory[*GetTenorStickerMappingResponse, GetTenorStickerMappingFn]()(
	func(a *api, sc session.Context, u factoryUtils[*GetTenorStickerMappingResponse]) (GetTenorStickerMappingFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("sticker"), "")
		serviceURL := u.MakeURL(base+"/api/message/sticker/tenor/mapping", nil, true)

		return func(ctx context.Context) (*GetTenorStickerMappingResponse, error) {
			payload := map[string]any{
				"imei": sc.IMEI(),
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.GetTenorStickerMapping", err)
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
