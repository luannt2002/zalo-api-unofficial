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
	ExtraGroupInfo struct {
		Purpose string   `json:"purpose"`
		Role    []string `json:"role"`
	}

	GetExtraGroupInfoResponse map[string]ExtraGroupInfo

	GetExtraGroupInfoFn = func(ctx context.Context, groupIDs ...string) (GetExtraGroupInfoResponse, error)
)

func (a *api) GetExtraGroupInfo(ctx context.Context, groupIDs ...string) (GetExtraGroupInfoResponse, error) {
	return a.e.GetExtraGroupInfo(ctx, groupIDs...)
}

var getExtraGroupInfoFactory = apiFactory[GetExtraGroupInfoResponse, GetExtraGroupInfoFn]()(
	func(a *api, sc session.Context, u factoryUtils[GetExtraGroupInfoResponse]) (GetExtraGroupInfoFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("group"), "")
		serviceURL := u.MakeURL(base+"/api/group/userctx/multiget", nil, true)

		return func(ctx context.Context, groupIDs ...string) (GetExtraGroupInfoResponse, error) {
			if len(groupIDs) == 0 {
				return nil, errs.NewZCA("missing group id", "api.GetExtraGroupInfo")
			}

			payload := map[string]any{
				"grids": groupIDs,
				"imei":  sc.IMEI(),
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.GetExtraGroupInfo", err)
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
