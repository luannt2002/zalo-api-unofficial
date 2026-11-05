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
	GiphyCategoryItem struct {
		ID       int      `json:"id"`
		Name     string   `json:"name"`
		EnName   string   `json:"enName"`
		Icon     string   `json:"icon"`
		Type     int      `json:"type"`
		Keywords []string `json:"keywords"`
	}
	GetGiphyCategoriesResponse struct {
		DataList []GiphyCategoryItem `json:"data_list"`
		HasMore  int                 `json:"has_more"`
	}
	GetGiphyCategoriesFn = func(ctx context.Context, offset, limit int) (*GetGiphyCategoriesResponse, error)
)

func (a *api) GetGiphyCategories(ctx context.Context, offset, limit int) (*GetGiphyCategoriesResponse, error) {
	return a.e.GetGiphyCategories(ctx, offset, limit)
}

var getGiphyCategoriesFactory = apiFactory[*GetGiphyCategoriesResponse, GetGiphyCategoriesFn]()(
	func(a *api, sc session.Context, u factoryUtils[*GetGiphyCategoriesResponse]) (GetGiphyCategoriesFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("sticker"), "")
		serviceURL := u.MakeURL(base+"/api/message/giphy/cates", nil, true)

		return func(ctx context.Context, offset, limit int) (*GetGiphyCategoriesResponse, error) {
			if limit <= 0 {
				limit = 10
			}
			if offset < 0 {
				offset = 0
			}

			payload := map[string]any{
				"offset": offset,
				"limit":  limit,
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.GetGiphyCategories", err)
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
