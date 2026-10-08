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
	CreateCatalogResponse struct {
		Item             model.CatalogItem `json:"item"`
		VersionLSCatalog int               `json:"version_ls_catalog"`
		VersionCatalog   int               `json:"version_catalog"`
	}
	CreateCatalogFn = func(ctx context.Context, name string) (*CreateCatalogResponse, error)
)

func (a *api) CreateCatalog(ctx context.Context, name string) (*CreateCatalogResponse, error) {
	return a.e.CreateCatalog(ctx, name)
}

var createCatalogFactory = apiFactory[*CreateCatalogResponse, CreateCatalogFn]()(
	func(a *api, sc session.Context, u factoryUtils[*CreateCatalogResponse]) (CreateCatalogFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("catalog"), "")
		serviceURL := u.MakeURL(base+"/api/prodcatalog/catalog/create", nil, true)

		return func(ctx context.Context, name string) (*CreateCatalogResponse, error) {
			payload := map[string]any{
				"catalog_name":  name,
				"catalog_photo": "",
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.CreateCatalog", err)
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
