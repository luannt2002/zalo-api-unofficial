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
	GiphyGifItem struct {
		ID       string         `json:"id"`
		Original model.GifMedia `json:"original"`
		Normal   model.GifMedia `json:"normal"`
		HD       model.GifMedia `json:"hd"`
		Preview  model.GifMedia `json:"preview"`
	}
	GetGiphyGifTrendingResponse struct {
		DataList []GiphyGifItem `json:"data_list"`
		HasMore  int            `json:"has_more"`
	}
	GetGiphyGifTrendingFn = func(ctx context.Context, offset, limit int) (*GetGiphyGifTrendingResponse, error)
)

func (a *api) GetGiphyGifTrending(ctx context.Context, offset, limit int) (*GetGiphyGifTrendingResponse, error) {
	return a.e.GetGiphyGifTrending(ctx, offset, limit)
}

var getGiphyGifTrendingFactory = apiFactory[*GetGiphyGifTrendingResponse, GetGiphyGifTrendingFn]()(
	func(a *api, sc session.Context, u factoryUtils[*GetGiphyGifTrendingResponse]) (GetGiphyGifTrendingFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("sticker"), "")
		serviceURL := u.MakeURL(base+"/api/message/giphy/trending", nil, true)

		return func(ctx context.Context, offset, limit int) (*GetGiphyGifTrendingResponse, error) {
			if limit <= 0 {
				limit = 10
			}
			if offset < 0 {
				offset = 0
			}

			payload := map[string]any{
				"offset": offset,
				"limit":  limit,
				"imei":   sc.IMEI(),
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.GetGiphyGifTrending", err)
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
