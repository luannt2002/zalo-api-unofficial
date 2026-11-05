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
	GetListBankAccountResponse struct {
		HasMore bool                `json:"hasMore"`
		Total   int                 `json:"total"`
		MyBanks []model.BankAccount `json:"myBanks"`
	}
	GetListBankAccountFn = func(ctx context.Context, page, limit int) (*GetListBankAccountResponse, error)
)

func (a *api) GetListBankAccount(ctx context.Context, page, limit int) (*GetListBankAccountResponse, error) {
	return a.e.GetListBankAccount(ctx, page, limit)
}

var getListBankAccountFactory = apiFactory[*GetListBankAccountResponse, GetListBankAccountFn]()(
	func(a *api, sc session.Context, u factoryUtils[*GetListBankAccountResponse]) (GetListBankAccountFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("zimsg"), "")
		serviceURL := u.MakeURL(base+"/api/transfer/list", nil, true)

		return func(ctx context.Context, page, limit int) (*GetListBankAccountResponse, error) {
			if limit <= 0 {
				limit = 20
			}
			if page < 0 {
				page = 0
			}

			payload := map[string]any{
				"page":  page,
				"limit": limit,
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.GetListBankAccount", err)
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
