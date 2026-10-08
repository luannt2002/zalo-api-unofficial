package api

import (
	"context"
	"net/http"
	"time"

	"github.com/luannt2002/zalo-api-unofficial/errs"
	"github.com/luannt2002/zalo-api-unofficial/internal/httpx"
	"github.com/luannt2002/zalo-api-unofficial/internal/jsonx"
	"github.com/luannt2002/zalo-api-unofficial/model"
	"github.com/luannt2002/zalo-api-unofficial/session"
)

type (
	CreateProductCatalogRequest struct {
		CatalogID   string `json:"catalogId"`
		ProductName string `json:"productName"`
		Price       string `json:"price"`
		Description string `json:"description"`

		// Up to 5 media files are allowed, ignored if ProductPhotos is provided
		Files []model.AttachmentSource `json:"files"`
		// List of product photo URLs, up to 5
		ProductPhotos []string `json:"product_photos"`
	}
	CreateProductCatalogResponse struct {
		Item             model.ProductCatalogItem `json:"item"`
		VersionLSCatalog int                      `json:"version_ls_catalog"`
		VersionCatalog   int                      `json:"version_catalog"`
	}
	CreateProductCatalogFn = func(ctx context.Context, options CreateProductCatalogRequest) (*CreateProductCatalogResponse, error)
)

func (a *api) CreateProductCatalog(ctx context.Context, options CreateProductCatalogRequest) (*CreateProductCatalogResponse, error) {
	return a.e.CreateProductCatalog(ctx, options)
}

var createProductCatalogFactory = apiFactory[*CreateProductCatalogResponse, CreateProductCatalogFn]()(
	func(a *api, sc session.Context, u factoryUtils[*CreateProductCatalogResponse]) (CreateProductCatalogFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("catalog"), "")
		serviceURL := u.MakeURL(base+"/api/prodcatalog/product/create", nil, true)

		return func(ctx context.Context, options CreateProductCatalogRequest) (*CreateProductCatalogResponse, error) {
			productPhotos := options.ProductPhotos
			if productPhotos == nil {
				productPhotos = make([]string, 0)
			}

			if len(options.ProductPhotos) == 0 && len(options.Files) > 0 {
				if len(options.Files) > 5 {
					return nil, errs.NewZCA("maximum 5 media files are allowed", "api.CreateProductCatalog")
				}

				for _, mediaFile := range options.Files {
					uploaded, err := a.UploadProductPhoto(ctx, mediaFile)
					if err != nil {
						return nil, err
					}

					url := uploaded.NormalURL
					if url == "" {
						url = uploaded.HdURL
					}
					productPhotos = append(productPhotos, url)
				}
			}

			if len(productPhotos) > 5 {
				return nil, errs.NewZCA("maximum 5 media files are allowed", "api.CreateProductCatalog")
			}

			payload := map[string]any{
				"product_name":   options.ProductName,
				"price":          options.Price,
				"description":    options.Description,
				"product_photos": productPhotos,
				"catalog_id":     options.CatalogID,
				"currency_unit":  "₫",
				"create_time":    time.Now().UnixMilli(),
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.CreateProductCatalog", err)
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
