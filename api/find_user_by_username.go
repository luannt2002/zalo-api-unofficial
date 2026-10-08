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

type AvatarSize int

const (
	AvatarSizeSmall AvatarSize = 120
	AvatarSizeLarge AvatarSize = 240
)

type UserBasic struct {
	Avatar      string                 `json:"avatar"`
	Cover       string                 `json:"cover"`
	Status      string                 `json:"status"`
	Gender      model.Gender           `json:"gender"`
	DOB         int64                  `json:"dob"`
	Sdob        string                 `json:"sdob"`
	GlobalID    string                 `json:"globalId"`
	BizPkg      model.ZBusinessPackage `json:"bizPkg"`
	UID         string                 `json:"uid"`
	ZaloName    string                 `json:"zalo_name"`
	DisplayName string                 `json:"display_name"`
}

type (
	FindUserByUsernameResponse = UserBasic
	FindUserByUsernameFn       = func(ctx context.Context, username string, avatarSize ...AvatarSize) (*FindUserByUsernameResponse, error)
)

func (a *api) FindUserByUsername(ctx context.Context, username string, avatarSize ...AvatarSize) (*FindUserByUsernameResponse, error) {
	return a.e.FindUserByUsername(ctx, username, avatarSize...)
}

var findUserByUsernameFactory = apiFactory[*FindUserByUsernameResponse, FindUserByUsernameFn]()(
	func(a *api, sc session.Context, u factoryUtils[*FindUserByUsernameResponse]) (FindUserByUsernameFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("friend"), "")
		serviceURL := u.MakeURL(base+"/api/friend/search/by-user-name", nil, true)

		return func(ctx context.Context, username string, avatarSize ...AvatarSize) (*FindUserByUsernameResponse, error) {
			size := AvatarSizeLarge
			if len(avatarSize) > 0 {
				size = avatarSize[0]
			}

			payload := map[string]any{
				"user_name":   username,
				"avatar_size": int(size),
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.FindUserByUsername", err)
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
