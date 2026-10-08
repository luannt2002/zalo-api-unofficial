package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/luannt2002/zalo-api-unofficial/errs"
	"github.com/luannt2002/zalo-api-unofficial/internal/httpx"
	"github.com/luannt2002/zalo-api-unofficial/internal/jsonx"
	"github.com/luannt2002/zalo-api-unofficial/session"
)

type (
	GroupMemberProfile struct {
		DisplayName    string `json:"displayName"`
		ZaloName       string `json:"zaloName"`
		Avatar         string `json:"avatar"`
		AccountStatus  int    `json:"accountStatus"`
		Type           int    `json:"type"`
		LastUpdateTime int64  `json:"lastUpdateTime"`
		GlobalID       string `json:"globalId"`
		ID             string `json:"id"`
	}

	GetGroupMembersInfoResponse struct {
		Profiles          map[string]GroupMemberProfile `json:"profiles"`
		UnchangedsProfile []any                         `json:"unchangeds_profile"`
	}
	GetGroupMembersInfoFn = func(ctx context.Context, memberID ...string) (*GetGroupMembersInfoResponse, error)
)

func (a *api) GetGroupMembersInfo(ctx context.Context, memberID ...string) (*GetGroupMembersInfoResponse, error) {
	return a.e.GetGroupMembersInfo(ctx, memberID...)
}

var getGroupMembersInfoFactory = apiFactory[*GetGroupMembersInfoResponse, GetGroupMembersInfoFn]()(
	func(a *api, sc session.Context, u factoryUtils[*GetGroupMembersInfoResponse]) (GetGroupMembersInfoFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("profile"), "")
		serviceURL := u.MakeURL(base+"/api/social/group/members", nil, true)

		return func(ctx context.Context, memberID ...string) (*GetGroupMembersInfoResponse, error) {
			friendPversionMap := make([]string, 0, len(memberID))
			for _, id := range memberID {
				if strings.HasSuffix(id, "_0") {
					friendPversionMap = append(friendPversionMap, id)
				} else {
					friendPversionMap = append(friendPversionMap, id+"_0")
				}
			}

			payload := map[string]any{
				"friend_pversion_map": friendPversionMap,
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.GetGroupMembersInfo", err)
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
