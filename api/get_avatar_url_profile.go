package api

import (
	"context"
	"net/http"

	"github.com/luannt2002/zalo-api-unofficial/errs"
	"github.com/luannt2002/zalo-api-unofficial/internal/httpx"
	"github.com/luannt2002/zalo-api-unofficial/internal/jsonx"
	"github.com/luannt2002/zalo-api-unofficial/session"
)

const avatarSizeLarge = 240

type (
	AvatarURLEntry struct {
		Avatar string `json:"avatar"`
	}

	GetAvatarUrlProfileResponse map[string]AvatarURLEntry
	GetAvatarUrlProfileFn       = func(ctx context.Context, avatarSize int, friendID ...string) (*GetAvatarUrlProfileResponse, error)
)

func (a *api) GetAvatarUrlProfile(ctx context.Context, avatarSize int, friendID ...string) (*GetAvatarUrlProfileResponse, error) {
	return a.e.GetAvatarUrlProfile(ctx, avatarSize, friendID...)
}

var getAvatarUrlProfileFactory = apiFactory[*GetAvatarUrlProfileResponse, GetAvatarUrlProfileFn]()(
	func(a *api, sc session.Context, u factoryUtils[*GetAvatarUrlProfileResponse]) (GetAvatarUrlProfileFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("profile"), "")
		serviceURL := u.MakeURL(base+"/api/social/profile/avatar-url", nil, true)

		return func(ctx context.Context, avatarSize int, friendID ...string) (*GetAvatarUrlProfileResponse, error) {
			if avatarSize <= 0 {
				avatarSize = avatarSizeLarge
			}

			payload := map[string]any{
				"friend_ids":  friendID,
				"avatar_size": avatarSize,
				"srcReq":      -1,
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.GetAvatarUrlProfile", err)
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
