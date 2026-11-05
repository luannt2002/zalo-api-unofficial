package api

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/luannt2002/zalo-api-unofficial/errs"
	"github.com/luannt2002/zalo-api-unofficial/internal/httpx"
	"github.com/luannt2002/zalo-api-unofficial/internal/jsonx"
	"github.com/luannt2002/zalo-api-unofficial/session"
)

type (
	ScanURLResponse struct {
		IsSafe bool `json:"isSafe"`
	}
	ScanURLFn = func(ctx context.Context, targetURL string) (*ScanURLResponse, error)
)

func (a *api) ScanURL(ctx context.Context, targetURL string) (*ScanURLResponse, error) {
	return a.e.ScanURL(ctx, targetURL)
}

var scanURLFactory = apiFactory[*ScanURLResponse, ScanURLFn]()(
	func(a *api, sc session.Context, u factoryUtils[*ScanURLResponse]) (ScanURLFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("file"), "")
		serviceURL := u.MakeURL(base+"/api/message/scanurl", nil, true)

		return func(ctx context.Context, targetURL string) (*ScanURLResponse, error) {
			if targetURL == "" {
				return nil, errs.NewZCA("missing url parameter", "api.ScanURL")
			}

			payload := map[string]any{
				"url": targetURL,
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.ScanURL", err)
			}

			form := url.Values{}
			form.Set("params", enc)

			resp, err := u.Request(ctx, serviceURL, &httpx.RequestOptions{
				Method: http.MethodPost,
				Body:   strings.NewReader(form.Encode()),
				Headers: http.Header{
					"Content-Type": []string{"application/x-www-form-urlencoded"},
				},
			})
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()

			return u.Resolve(resp, true)
		}, nil
	},
)
