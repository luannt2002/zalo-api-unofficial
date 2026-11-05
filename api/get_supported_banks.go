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
	GetSupportedBanksResponse struct {
		Banks []model.BankInfo `json:"banks"`
	}
	GetSupportedBanksFn = func(ctx context.Context) (*GetSupportedBanksResponse, error)
)

func (a *api) GetSupportedBanks(ctx context.Context) (*GetSupportedBanksResponse, error) {
	return a.e.GetSupportedBanks(ctx)
}

var getSupportedBanksFactory = apiFactory[*GetSupportedBanksResponse, GetSupportedBanksFn]()(
	func(a *api, sc session.Context, u factoryUtils[*GetSupportedBanksResponse]) (GetSupportedBanksFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("zimsg"), "")
		serviceURL := u.MakeURL(base+"/api/transfer/conf", nil, true)

		return func(ctx context.Context) (*GetSupportedBanksResponse, error) {
			payload := map[string]any{}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.GetSupportedBanks", err)
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
