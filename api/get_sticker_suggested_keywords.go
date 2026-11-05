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
	GetStickerSuggestedKeywordsResponse struct {
		Keywords    []string `json:"keywords"`
		ExpiredTime int64    `json:"expired_time"`
		WordSearch  int      `json:"word_search"`
	}
	GetStickerSuggestedKeywordsFn = func(ctx context.Context) (*GetStickerSuggestedKeywordsResponse, error)
)

func (a *api) GetStickerSuggestedKeywords(ctx context.Context) (*GetStickerSuggestedKeywordsResponse, error) {
	return a.e.GetStickerSuggestedKeywords(ctx)
}

var getStickerSuggestedKeywordsFactory = apiFactory[*GetStickerSuggestedKeywordsResponse, GetStickerSuggestedKeywordsFn]()(
	func(a *api, sc session.Context, u factoryUtils[*GetStickerSuggestedKeywordsResponse]) (GetStickerSuggestedKeywordsFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("sticker"), "")
		serviceURL := u.MakeURL(base+"/api/message/sticker/suggest/keywords", nil, true)

		return func(ctx context.Context) (*GetStickerSuggestedKeywordsResponse, error) {
			payload := map[string]any{
				"imei": sc.IMEI(),
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.GetStickerSuggestedKeywords", err)
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
