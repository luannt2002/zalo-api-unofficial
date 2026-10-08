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
	GetProductCatalogListRequest struct {
		CatalogID      string `json:"catalogId"`
		Limit          int    `json:"limit"`
		VersionCatalog int    `json:"versionCatalog"`
		LastProductID  string `json:"lastProductId"`
		Page           int    `json:"page"`
	}
	GetProductCatalogListResponse struct {
		Items   []model.ProductCatalogItem `json:"items"`
		Version int                        `json:"version"`
		HasMore int                        `json:"has_more"`
	}
	GetProductCatalogListFn = func(ctx context.Context, options GetProductCatalogListRequest) (*GetProductCatalogListResponse, error)
)

func (a *api) GetProductCatalogList(ctx context.Context, options GetProductCatalogListRequest) (*GetProductCatalogListResponse, error) {
	return a.e.GetProductCatalogList(ctx, options)
}

var getProductCatalogListFactory = apiFactory[*GetProductCatalogListResponse, GetProductCatalogListFn]()(
	func(a *api, sc session.Context, u factoryUtils[*GetProductCatalogListResponse]) (GetProductCatalogListFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("catalog"), "")
		serviceURL := u.MakeURL(base+"/api/prodcatalog/product/list", nil, true)

		return func(ctx context.Context, options GetProductCatalogListRequest) (*GetProductCatalogListResponse, error) {
			if options.Limit <= 0 {
				options.Limit = 100
			}
			lastProductID := options.LastProductID
			if lastProductID == "" {
				lastProductID = "-1"
			}

			payload := map[string]any{
				"catalog_id":      options.CatalogID,
				"limit":           options.Limit,
				"version_catalog": options.VersionCatalog,
				"last_product_id": lastProductID,
				"page":            options.Page,
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.GetProductCatalogList", err)
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
