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
	LostFocusResponse struct {
		Status bool `json:"status"`
	}
	LostFocusFn = func(ctx context.Context) (*LostFocusResponse, error)
)

func (a *api) LostFocus(ctx context.Context) (*LostFocusResponse, error) {
	return a.e.LostFocus(ctx)
}

var lostFocusFactory = apiFactory[*LostFocusResponse, LostFocusFn]()(
	func(a *api, sc session.Context, u factoryUtils[*LostFocusResponse]) (LostFocusFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("profile"), "")
		serviceURL := u.MakeURL(base+"/api/social/profile/changefgtobg", nil, true)

		return func(ctx context.Context) (*LostFocusResponse, error) {
			payload := map[string]any{}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.LostFocus", err)
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
