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
	GiphyDictionaryItem struct {
		ID         string   `json:"id"`
		EnKeywords []string `json:"enKeywords"`
		ViKeywords []string `json:"viKeywords"`
	}
	GetGiphyGifDictionariesResponse struct {
		DataList []GiphyDictionaryItem `json:"data_list"`
		HasMore  int                   `json:"has_more"`
	}
	GetGiphyGifDictionariesFn = func(ctx context.Context, offset, limit int) (*GetGiphyGifDictionariesResponse, error)
)

func (a *api) GetGiphyGifDictionaries(ctx context.Context, offset, limit int) (*GetGiphyGifDictionariesResponse, error) {
	return a.e.GetGiphyGifDictionaries(ctx, offset, limit)
}

var getGiphyGifDictionariesFactory = apiFactory[*GetGiphyGifDictionariesResponse, GetGiphyGifDictionariesFn]()(
	func(a *api, sc session.Context, u factoryUtils[*GetGiphyGifDictionariesResponse]) (GetGiphyGifDictionariesFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("sticker"), "")
		serviceURL := u.MakeURL(base+"/api/message/giphy/dicts", nil, true)

		return func(ctx context.Context, offset, limit int) (*GetGiphyGifDictionariesResponse, error) {
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
				return nil, errs.WrapZCA("failed to encrypt params", "api.GetGiphyGifDictionaries", err)
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
