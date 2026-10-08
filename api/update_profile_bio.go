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
	UpdateProfileBioResponse = string
	UpdateProfileBioFn       = func(ctx context.Context, bio string) (UpdateProfileBioResponse, error)
)

func (a *api) UpdateProfileBio(ctx context.Context, bio string) (UpdateProfileBioResponse, error) {
	return a.e.UpdateProfileBio(ctx, bio)
}

var updateProfileBioFactory = apiFactory[UpdateProfileBioResponse, UpdateProfileBioFn]()(
	func(a *api, sc session.Context, u factoryUtils[UpdateProfileBioResponse]) (UpdateProfileBioFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("profile"), "")
		serviceURL := u.MakeURL(base+"/api/social/profile/status", nil, true)

		return func(ctx context.Context, bio string) (UpdateProfileBioResponse, error) {
			payload := map[string]any{
				"status": bio,
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return "", errs.WrapZCA("failed to encrypt params", "api.UpdateProfileBio", err)
			}

			body := httpx.BuildFormBody(map[string]string{"params": enc})
			resp, err := u.Request(ctx, serviceURL, &httpx.RequestOptions{
				Method: http.MethodPost,
				Body:   body,
			})
			if err != nil {
				return "", err
			}
			defer resp.Body.Close()

			return u.Resolve(resp, true)
		}, nil
	},
)
