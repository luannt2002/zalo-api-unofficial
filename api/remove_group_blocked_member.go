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
	RemoveGroupBlockedMemberResponse = string
	RemoveGroupBlockedMemberFn       = func(ctx context.Context, groupID string, memberID ...string) (RemoveGroupBlockedMemberResponse, error)
)

func (a *api) RemoveGroupBlockedMember(ctx context.Context, groupID string, memberID ...string) (RemoveGroupBlockedMemberResponse, error) {
	return a.e.RemoveGroupBlockedMember(ctx, groupID, memberID...)
}

var removeGroupBlockedMemberFactory = apiFactory[RemoveGroupBlockedMemberResponse, RemoveGroupBlockedMemberFn]()(
	func(a *api, sc session.Context, u factoryUtils[RemoveGroupBlockedMemberResponse]) (RemoveGroupBlockedMemberFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("group"), "")
		serviceURL := u.MakeURL(base+"/api/group/blockedmems/remove", nil, true)

		return func(ctx context.Context, groupID string, memberID ...string) (RemoveGroupBlockedMemberResponse, error) {
			payload := map[string]any{
				"grid":    groupID,
				"members": memberID,
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return "", errs.WrapZCA("failed to encrypt params", "api.RemoveGroupBlockedMember", err)
			}

			url := u.MakeURL(serviceURL, map[string]any{"params": enc}, true)
			resp, err := u.Request(ctx, url, &httpx.RequestOptions{Method: http.MethodGet})
			if err != nil {
				return "", err
			}
			defer resp.Body.Close()

			return u.Resolve(resp, true)
		}, nil
	},
)
