package api

import (
	"context"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/luannt2002/zalo-api-unofficial/errs"
	"github.com/luannt2002/zalo-api-unofficial/internal/httpx"
	"github.com/luannt2002/zalo-api-unofficial/internal/jsonx"
	"github.com/luannt2002/zalo-api-unofficial/model"
	"github.com/luannt2002/zalo-api-unofficial/session"
)

type (
	UploadProductPhotoResponse struct {
		NormalURL    string `json:"normalUrl"`
		PhotoID      string `json:"photoId"`
		Finished     int    `json:"finished"`
		HdURL        string `json:"hdUrl"`
		ThumbURL     string `json:"thumbUrl"`
		ClientFileID int    `json:"clientFileId"`
		ChunkID      int    `json:"chunkId"`
	}
	UploadProductPhotoFn = func(ctx context.Context, source model.AttachmentSource) (*UploadProductPhotoResponse, error)
)

func (a *api) UploadProductPhoto(ctx context.Context, source model.AttachmentSource) (*UploadProductPhotoResponse, error) {
	return a.e.UploadProductPhoto(ctx, source)
}

var uploadProductPhotoFactory = apiFactory[*UploadProductPhotoResponse, UploadProductPhotoFn]()(
	func(a *api, sc session.Context, u factoryUtils[*UploadProductPhotoResponse]) (UploadProductPhotoFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("file"), "")
		serviceURL := u.MakeURL(base+"/api/product/upload/photo", nil, true)

		return func(ctx context.Context, source model.AttachmentSource) (*UploadProductPhotoResponse, error) {
			var (
				reader       io.Reader
				closer       io.Closer
				fileName     string
				fileMetadata model.AttachmentMetadata
			)

			defer func() {
				if closer != nil {
					_ = closer.Close()
				}
			}()

			if f := source.String(); f != "" {
				r, err := os.Open(f)
				if err != nil {
					return nil, errs.WrapZCA("failed to read file", "api.UploadProductPhoto", err)
				}

				reader = r
				closer = r

				fileMetadata, fileName, err = sc.GetImageMetadata(f)
				if err != nil {
					return nil, err
				}
			} else if f := source.Object(); f != nil {
				reader, fileName, fileMetadata = f.Data, f.Filename, f.Metadata
			}

			forms, err := httpx.BuildFormData(
				"chunkContent", reader,
				httpx.WithContentType("application/octet-stream"),
				httpx.WithFileName(fileName),
			)
			if err != nil || len(forms) == 0 || forms[0] == nil {
				return nil, errs.WrapZCA("failed to build form data", "api.UploadProductPhoto", err)
			}
			formData := forms[0]

			payload := map[string]any{
				"totalChunk": 1,
				"fileName":   fileName,
				"clientId":   time.Now().UnixMilli(),
				"totalSize":  fileMetadata.Size,
				"imei":       sc.IMEI(),
				"chunkId":    1,
				"toid":       sc.LoginInfo().Send2meID,
				"featureId":  1,
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.UploadProductPhoto", err)
			}

			url := u.MakeURL(serviceURL, map[string]any{"params": enc}, true)
			resp, err := u.Request(ctx, url, &httpx.RequestOptions{
				Method:  http.MethodPost,
				Headers: formData.Header,
				Body:    formData.Body,
			})
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()

			return u.Resolve(resp, true)
		}, nil
	},
)
