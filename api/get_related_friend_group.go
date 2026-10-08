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
	GetRelatedFriendGroupResponse struct {
		GroupRelateds map[string][]string `json:"groupRelateds"`
	}
	GetRelatedFriendGroupFn = func(ctx context.Context, friendID ...string) (*GetRelatedFriendGroupResponse, error)
)

func (a *api) GetRelatedFriendGroup(ctx context.Context, friendID ...string) (*GetRelatedFriendGroupResponse, error) {
	return a.e.GetRelatedFriendGroup(ctx, friendID...)
}

var getRelatedFriendGroupFactory = apiFactory[*GetRelatedFriendGroupResponse, GetRelatedFriendGroupFn]()(
	func(a *api, sc session.Context, u factoryUtils[*GetRelatedFriendGroupResponse]) (GetRelatedFriendGroupFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("friend"), "")
		serviceURL := u.MakeURL(base+"/api/friend/group/related", nil, true)

		return func(ctx context.Context, friendID ...string) (*GetRelatedFriendGroupResponse, error) {
			friendIDs := friendID
			if friendIDs == nil {
				friendIDs = []string{}
			}

			payload := map[string]any{
				"friend_ids": jsonx.Stringify(friendIDs),
				"imei":       sc.IMEI(),
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.GetRelatedFriendGroup", err)
			}

			body := httpx.BuildFormBody(map[string]string{"params": enc})
			resp, err := u.Request(ctx, serviceURL, &httpx.RequestOptions{
				Method: http.MethodPost,
				Body:   body,
			})
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()

			return u.Resolve(resp, true)
		}, nil
	},
)
