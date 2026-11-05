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
	SearchGiphyGifResponse struct {
		DataList []GiphyGifItem `json:"data_list"`
		HasMore  int            `json:"has_more"`
	}
	SearchGiphyGifFn = func(ctx context.Context, keyword string, offset, limit int) (*SearchGiphyGifResponse, error)
)

func (a *api) SearchGiphyGif(ctx context.Context, keyword string, offset, limit int) (*SearchGiphyGifResponse, error) {
	return a.e.SearchGiphyGif(ctx, keyword, offset, limit)
}

var searchGiphyGifFactory = apiFactory[*SearchGiphyGifResponse, SearchGiphyGifFn]()(
	func(a *api, sc session.Context, u factoryUtils[*SearchGiphyGifResponse]) (SearchGiphyGifFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("sticker"), "")
		serviceURL := u.MakeURL(base+"/api/message/giphy/search", nil, true)

		return func(ctx context.Context, keyword string, offset, limit int) (*SearchGiphyGifResponse, error) {
			if limit <= 0 {
				limit = 10
			}
			if offset < 0 {
				offset = 0
			}

			payload := map[string]any{
				"keyword": keyword,
				"offset":  offset,
				"limit":   limit,
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.SearchGiphyGif", err)
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
