package api

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/luannt2002/zalo-api-unofficial/errs"
	"github.com/luannt2002/zalo-api-unofficial/internal/httpx"
	"github.com/luannt2002/zalo-api-unofficial/internal/jsonx"
	"github.com/luannt2002/zalo-api-unofficial/session"
)

type (
	ExtraFriendInfo struct {
		Purpose string   `json:"purpose"`
		Role    []string `json:"role"`
	}

	GetExtraFriendInfoResponse map[string]ExtraFriendInfo

	GetExtraFriendInfoFn = func(ctx context.Context, friendIDs ...string) (GetExtraFriendInfoResponse, error)
)

func (a *api) GetExtraFriendInfo(ctx context.Context, friendIDs ...string) (GetExtraFriendInfoResponse, error) {
	return a.e.GetExtraFriendInfo(ctx, friendIDs...)
}

var getExtraFriendInfoFactory = apiFactory[GetExtraFriendInfoResponse, GetExtraFriendInfoFn]()(
	func(a *api, sc session.Context, u factoryUtils[GetExtraFriendInfoResponse]) (GetExtraFriendInfoFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("friend"), jsonx.FirstOr(sc.GetZpwService("profile"), ""))
		serviceURL := u.MakeURL(base+"/api/friend/context/multiget", nil, true)

		return func(ctx context.Context, friendIDs ...string) (GetExtraFriendInfoResponse, error) {
			if len(friendIDs) == 0 {
				return nil, errs.NewZCA("missing friend id", "api.GetExtraFriendInfo")
			}

			payload := map[string]any{
				"fids": friendIDs,
				"imei": sc.IMEI(),
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.GetExtraFriendInfo", err)
			}

			form := url.Values{}
			form.Set("params", enc)

			resp, err := u.Request(ctx, serviceURL, &httpx.RequestOptions{
				Method: http.MethodPost,
				Body:   strings.NewReader(form.Encode()),
				Headers: http.Header{
					"Content-Type": []string{"application/x-www-form-urlencoded"},
				},
			})
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()

			return u.Resolve(resp, true)
		}, nil
	},
)
