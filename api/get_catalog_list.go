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
	GetCatalogListRequest struct {
		Limit         int `json:"limit"`
		LastProductID int `json:"lastProductId"`
		Page          int `json:"page"`
	}
	GetCatalogListResponse struct {
		Items   []model.CatalogItem `json:"items"`
		Version int                 `json:"version"`
		HasMore int                 `json:"has_more"`
	}
	GetCatalogListFn = func(ctx context.Context, options GetCatalogListRequest) (*GetCatalogListResponse, error)
)

func (a *api) GetCatalogList(ctx context.Context, options GetCatalogListRequest) (*GetCatalogListResponse, error) {
	return a.e.GetCatalogList(ctx, options)
}

var getCatalogListFactory = apiFactory[*GetCatalogListResponse, GetCatalogListFn]()(
	func(a *api, sc session.Context, u factoryUtils[*GetCatalogListResponse]) (GetCatalogListFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("catalog"), "")
		serviceURL := u.MakeURL(base+"/api/prodcatalog/catalog/list", nil, true)

		return func(ctx context.Context, options GetCatalogListRequest) (*GetCatalogListResponse, error) {
			if options.Limit <= 0 {
				options.Limit = 20
			}
			if options.LastProductID == 0 {
				options.LastProductID = -1
			}

			payload := map[string]any{
				"version_list_catalog": 0,
				"limit":                options.Limit,
				"last_product_id":      options.LastProductID,
				"page":                 options.Page,
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.GetCatalogList", err)
			}

			body := httpx.BuildFormBody(map[string]string{"params": enc})
			resp, err := u.Request(ctx, serviceURL, &httpx.RequestOptions{
				Method: http.MethodPost,
				Body:   body,
			})
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()

			return u.Resolve(resp, true)
		}, nil
	},
)
