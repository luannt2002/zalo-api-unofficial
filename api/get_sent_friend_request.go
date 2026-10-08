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
	FriendRequestInfo struct {
		Message string `json:"message"`
		Src     int    `json:"src"`
		Time    int    `json:"time"`
	}
	SentFriendRequestInfo struct {
		UserID      string                 `json:"userId"`
		ZaloName    string                 `json:"zaloName"`
		DisplayName string                 `json:"displayName"`
		Avatar      string                 `json:"avatar"`
		GlobalID    string                 `json:"globalId"`
		BizPkg      model.ZBusinessPackage `json:"bizPkg"`
		FReqInfo    FriendRequestInfo      `json:"fReqInfo"`
	}

	GetSentFriendRequestResponse map[string]SentFriendRequestInfo
	GetSentFriendRequestFn       = func(ctx context.Context) (*GetSentFriendRequestResponse, error)
)

func (a *api) GetSentFriendRequest(ctx context.Context) (*GetSentFriendRequestResponse, error) {
	return a.e.GetSentFriendRequest(ctx)
}

var getSentFriendRequestFactory = apiFactory[*GetSentFriendRequestResponse, GetSentFriendRequestFn]()(
	func(a *api, sc session.Context, u factoryUtils[*GetSentFriendRequestResponse]) (GetSentFriendRequestFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("friend"), "")
		serviceURL := u.MakeURL(base+"/api/friend/requested/list", nil, true)

		return func(ctx context.Context) (*GetSentFriendRequestResponse, error) {
			payload := map[string]any{
				"imei": sc.IMEI(),
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.GetSentFriendRequest", err)
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
