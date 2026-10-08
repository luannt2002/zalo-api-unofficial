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
	GetFullAvatarResponse struct {
		BkFullAvatar string `json:"bk_full_avatar"`
		FullAvatar   string `json:"full_avatar"`
	}
	GetFullAvatarFn = func(ctx context.Context, friendID string) (*GetFullAvatarResponse, error)
)

func (a *api) GetFullAvatar(ctx context.Context, friendID string) (*GetFullAvatarResponse, error) {
	return a.e.GetFullAvatar(ctx, friendID)
}

var getFullAvatarFactory = apiFactory[*GetFullAvatarResponse, GetFullAvatarFn]()(
	func(a *api, sc session.Context, u factoryUtils[*GetFullAvatarResponse]) (GetFullAvatarFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("profile"), "")
		serviceURL := u.MakeURL(base+"/api/social/profile/avatar", nil, true)

		return func(ctx context.Context, friendID string) (*GetFullAvatarResponse, error) {
			payload := map[string]any{
				"fid":  friendID,
				"imei": sc.IMEI(),
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.GetFullAvatar", err)
			}

			url := u.MakeURL(serviceURL, map[string]any{"params": enc}, true)
			resp, err := u.Request(ctx, url, &httpx.RequestOptions{Method: http.MethodGet})
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()

			return u.Resolve(resp, true)
		}, nil
	},
)
