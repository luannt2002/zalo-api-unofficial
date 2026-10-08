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
	UpgradeGroupToCommunityResponse = string
	UpgradeGroupToCommunityFn       = func(ctx context.Context, groupID string) (UpgradeGroupToCommunityResponse, error)
)

func (a *api) UpgradeGroupToCommunity(ctx context.Context, groupID string) (UpgradeGroupToCommunityResponse, error) {
	return a.e.UpgradeGroupToCommunity(ctx, groupID)
}

var upgradeGroupToCommunityFactory = apiFactory[UpgradeGroupToCommunityResponse, UpgradeGroupToCommunityFn]()(
	func(a *api, sc session.Context, u factoryUtils[UpgradeGroupToCommunityResponse]) (UpgradeGroupToCommunityFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("group"), "")
		serviceURL := u.MakeURL(base+"/api/group/upgrade/community", nil, true)

		return func(ctx context.Context, groupID string) (UpgradeGroupToCommunityResponse, error) {
			payload := map[string]any{
				"grId":     groupID,
				"language": sc.Language(),
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return "", errs.WrapZCA("failed to encrypt params", "api.UpgradeGroupToCommunity", err)
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
