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
	DeleteProductCatalogRequest struct {
		ProductIDs []string `json:"productIds"`
		CatalogID  string   `json:"catalogId"`
	}
	DeleteProductCatalogResponse struct {
		Item             []int `json:"item"`
		VersionLSCatalog int   `json:"version_ls_catalog"`
		VersionCatalog   int   `json:"version_catalog"`
	}
	DeleteProductCatalogFn = func(ctx context.Context, options DeleteProductCatalogRequest) (*DeleteProductCatalogResponse, error)
)

func (a *api) DeleteProductCatalog(ctx context.Context, options DeleteProductCatalogRequest) (*DeleteProductCatalogResponse, error) {
	return a.e.DeleteProductCatalog(ctx, options)
}

var deleteProductCatalogFactory = apiFactory[*DeleteProductCatalogResponse, DeleteProductCatalogFn]()(
	func(a *api, sc session.Context, u factoryUtils[*DeleteProductCatalogResponse]) (DeleteProductCatalogFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("catalog"), "")
		serviceURL := u.MakeURL(base+"/api/prodcatalog/product/mdelete", nil, true)

		return func(ctx context.Context, options DeleteProductCatalogRequest) (*DeleteProductCatalogResponse, error) {
			productIDs := options.ProductIDs
			if productIDs == nil {
				productIDs = make([]string, 0)
			}

			payload := map[string]any{
				"product_ids": productIDs,
				"catalog_id":  options.CatalogID,
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.DeleteProductCatalog", err)
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
