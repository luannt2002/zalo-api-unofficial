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

type (
	GetCloseFriendsResponse []model.User
	GetCloseFriendsFn       = func(ctx context.Context) (*GetCloseFriendsResponse, error)
)

func (a *api) GetCloseFriends(ctx context.Context) (*GetCloseFriendsResponse, error) {
	return a.e.GetCloseFriends(ctx)
}

var getCloseFriendsFactory = apiFactory[*GetCloseFriendsResponse, GetCloseFriendsFn]()(
	func(a *api, sc session.Context, u factoryUtils[*GetCloseFriendsResponse]) (GetCloseFriendsFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("profile"), "")
		serviceURL := u.MakeURL(base+"/api/social/friend/getclosedfriends", nil, true)

		return func(ctx context.Context) (*GetCloseFriendsResponse, error) {
			payload := map[string]any{}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.GetCloseFriends", err)
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
