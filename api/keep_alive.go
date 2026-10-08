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
	KeepAliveResponse struct {
		// ConfigVesion mirrors the (misspelled) field name returned by Zalo.
		ConfigVesion int `json:"config_vesion"`
	}
	KeepAliveFn = func(ctx context.Context) (*KeepAliveResponse, error)
)

func (a *api) KeepAlive(ctx context.Context) (*KeepAliveResponse, error) {
	return a.e.KeepAlive(ctx)
}

var keepAliveFactory = apiFactory[*KeepAliveResponse, KeepAliveFn]()(
	func(a *api, sc session.Context, u factoryUtils[*KeepAliveResponse]) (KeepAliveFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("chat"), "")
		serviceURL := u.MakeURL(base+"/keepalive", nil, true)

		return func(ctx context.Context) (*KeepAliveResponse, error) {
			payload := map[string]any{
				"imei": sc.IMEI(),
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.KeepAlive", err)
			}

			url := u.MakeURL(serviceURL, map[string]any{"params": enc}, true)
			resp, err := u.Request(ctx, url, &httpx.RequestOptions{Method: http.MethodGet})
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()

			return u.Resolve(resp, false)
		}, nil
	},
)
