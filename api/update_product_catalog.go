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
	UpdateProductCatalogRequest struct {
		CatalogID   string `json:"catalogId"`
		ProductID   string `json:"productId"`
		ProductName string `json:"productName"`
		Price       string `json:"price"`
		Description string `json:"description"`
		CreateTime  int64  `json:"createTime"`

		// Up to 5 media files are allowed, ignored if ProductPhotos is provided
		Files []model.AttachmentSource `json:"files"`
		// List of product photo URLs, up to 5
		ProductPhotos []string `json:"product_photos"`
	}
	UpdateProductCatalogResponse struct {
		Item             model.ProductCatalogItem `json:"item"`
		VersionLSCatalog int                      `json:"version_ls_catalog"`
		VersionCatalog   int                      `json:"version_catalog"`
	}
	UpdateProductCatalogFn = func(ctx context.Context, options UpdateProductCatalogRequest) (*UpdateProductCatalogResponse, error)
)

func (a *api) UpdateProductCatalog(ctx context.Context, options UpdateProductCatalogRequest) (*UpdateProductCatalogResponse, error) {
	return a.e.UpdateProductCatalog(ctx, options)
}

var updateProductCatalogFactory = apiFactory[*UpdateProductCatalogResponse, UpdateProductCatalogFn]()(
	func(a *api, sc session.Context, u factoryUtils[*UpdateProductCatalogResponse]) (UpdateProductCatalogFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("catalog"), "")
		serviceURL := u.MakeURL(base+"/api/prodcatalog/product/update", nil, true)

		return func(ctx context.Context, options UpdateProductCatalogRequest) (*UpdateProductCatalogResponse, error) {
			productPhotos := options.ProductPhotos
			if productPhotos == nil {
				productPhotos = make([]string, 0)
			}

			if len(options.ProductPhotos) == 0 && len(options.Files) > 0 {
				if len(options.Files) > 5 {
					return nil, errs.NewZCA("maximum 5 media files are allowed", "api.UpdateProductCatalog")
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
				return nil, errs.NewZCA("maximum 5 media files are allowed", "api.UpdateProductCatalog")
			}

			payload := map[string]any{
				"product_id":     options.ProductID,
				"product_name":   options.ProductName,
				"price":          options.Price,
				"description":    options.Description,
				"product_photos": productPhotos,
				"catalog_id":     options.CatalogID,
				"currency_unit":  "₫",
				"create_time":    options.CreateTime,
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.UpdateProductCatalog", err)
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
