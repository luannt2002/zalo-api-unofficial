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
	EncryptGlobalIDRequest struct {
		UIDs []string `json:"uids"`
	}

	EncryptGlobalIDResponse map[string]string

	EncryptGlobalIDFn = func(ctx context.Context, req EncryptGlobalIDRequest) (EncryptGlobalIDResponse, error)
)

func (r *EncryptGlobalIDResponse) UnmarshalJSON(b []byte) error {
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

func (a *api) EncryptGlobalID(ctx context.Context, req EncryptGlobalIDRequest) (EncryptGlobalIDResponse, error) {
	return a.e.EncryptGlobalID(ctx, req)
}

var encryptGlobalIDFactory = apiFactory[EncryptGlobalIDResponse, EncryptGlobalIDFn]()(
	func(a *api, sc session.Context, u factoryUtils[EncryptGlobalIDResponse]) (EncryptGlobalIDFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("profile"), "")
		serviceURL := u.MakeURL(base+"/api/gid/encrypt", nil, true)

		return func(ctx context.Context, req EncryptGlobalIDRequest) (EncryptGlobalIDResponse, error) {
			if len(req.UIDs) == 0 {
				return nil, errs.NewZCA("uids cannot be empty", "api.EncryptGlobalID")
			}

			payload := map[string]any{
				"noiseUids": jsonx.Stringify(req.UIDs),
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.EncryptGlobalID", err)
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
