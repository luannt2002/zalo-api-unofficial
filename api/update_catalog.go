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
	UpdateCatalogRequest struct {
		CatalogID   string `json:"catalogId"`
		CatalogName string `json:"catalogName"`
	}
	UpdateCatalogResponse struct {
		Item             model.CatalogItem `json:"item"`
		VersionLSCatalog int               `json:"version_ls_catalog"`
		VersionCatalog   int               `json:"version_catalog"`
	}
	UpdateCatalogFn = func(ctx context.Context, options UpdateCatalogRequest) (*UpdateCatalogResponse, error)
)

func (a *api) UpdateCatalog(ctx context.Context, options UpdateCatalogRequest) (*UpdateCatalogResponse, error) {
	return a.e.UpdateCatalog(ctx, options)
}

var updateCatalogFactory = apiFactory[*UpdateCatalogResponse, UpdateCatalogFn]()(
	func(a *api, sc session.Context, u factoryUtils[*UpdateCatalogResponse]) (UpdateCatalogFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("catalog"), "")
		serviceURL := u.MakeURL(base+"/api/prodcatalog/catalog/update", nil, true)

		return func(ctx context.Context, options UpdateCatalogRequest) (*UpdateCatalogResponse, error) {
			payload := map[string]any{
				"catalog_id":    options.CatalogID,
				"catalog_name":  options.CatalogName,
				"catalog_photo": "",
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.UpdateCatalog", err)
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
