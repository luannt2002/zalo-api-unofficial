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
	GetBizAccountBiz struct {
		Desc    *string                `json:"desc"`
		Cate    model.BusinessCategory `json:"cate"`
		Addr    string                 `json:"addr"`
		Website string                 `json:"website"`
		Email   string                 `json:"email"`
	}
	GetBizAccountSettingStartPage struct {
		EnableBizLabel int `json:"enable_biz_label"`
		EnableCate     int `json:"enable_cate"`
		EnableAdd      int `json:"enable_add"`
		CtaProfile     int `json:"cta_profile"`
		// Relative path used to build the catalog URL.
		//
		// Example: https://catalog.zalo.me/{cta_catalog}
		CtaCatalog *string `json:"cta_catalog"`
	}
	GetBizAccountResponse struct {
		Biz              *GetBizAccountBiz              `json:"biz"`
		SettingStartPage *GetBizAccountSettingStartPage `json:"setting_start_page"`
		PkgID            int                            `json:"pkgId"`
	}
	GetBizAccountFn = func(ctx context.Context, friendID string) (*GetBizAccountResponse, error)
)

func (a *api) GetBizAccount(ctx context.Context, friendID string) (*GetBizAccountResponse, error) {
	return a.e.GetBizAccount(ctx, friendID)
}

var getBizAccountFactory = apiFactory[*GetBizAccountResponse, GetBizAccountFn]()(
	func(a *api, sc session.Context, u factoryUtils[*GetBizAccountResponse]) (GetBizAccountFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("profile"), "")
		serviceURL := u.MakeURL(base+"/api/social/friend/get-bizacc", nil, true)

		return func(ctx context.Context, friendID string) (*GetBizAccountResponse, error) {
			payload := map[string]any{
				"fid": friendID,
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.GetBizAccount", err)
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
