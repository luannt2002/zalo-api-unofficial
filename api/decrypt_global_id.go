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
	DecryptGlobalIDRequest struct {
		GlobalIDs []string `json:"globalIds"`
	}

	DecryptGlobalIDResponse map[string]string

	DecryptGlobalIDFn = func(ctx context.Context, req DecryptGlobalIDRequest) (DecryptGlobalIDResponse, error)
)

func (r *DecryptGlobalIDResponse) UnmarshalJSON(b []byte) error {
	var m map[string]string
	if err := jsonx.UnmarshalLenient(b, &m); err == nil && m != nil {
		*r = m
		return nil
	}
	var nested struct {
		Data map[string]string `json:"data"`
	}
	if err := jsonx.UnmarshalLenient(b, &nested); err == nil && nested.Data != nil {
		*r = nested.Data
		return nil
	}
	*r = make(map[string]string)
	return nil
}

func (a *api) DecryptGlobalID(ctx context.Context, req DecryptGlobalIDRequest) (DecryptGlobalIDResponse, error) {
	return a.e.DecryptGlobalID(ctx, req)
}

var decryptGlobalIDFactory = apiFactory[DecryptGlobalIDResponse, DecryptGlobalIDFn]()(
	func(a *api, sc session.Context, u factoryUtils[DecryptGlobalIDResponse]) (DecryptGlobalIDFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("profile"), "")
		serviceURL := u.MakeURL(base+"/api/gid/decrypt", nil, true)

		return func(ctx context.Context, req DecryptGlobalIDRequest) (DecryptGlobalIDResponse, error) {
			if len(req.GlobalIDs) == 0 {
				return nil, errs.NewZCA("globalIds cannot be empty", "api.DecryptGlobalID")
			}

			payload := map[string]any{
				"globalUids": jsonx.Stringify(req.GlobalIDs),
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.DecryptGlobalID", err)
			}

			url := u.MakeURL(serviceURL, map[string]any{"params": enc}, true)
			resp, err := u.Request(ctx, url, &httpx.RequestOptions{Method: http.MethodPost})
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()

			return u.Resolve(resp, true)
		}, nil
	},
)
