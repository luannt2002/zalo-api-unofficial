package api

import (
	"context"
	"net/http"

	"github.com/luannt2002/zalo-api-unofficial/errs"
	"github.com/luannt2002/zalo-api-unofficial/internal/httpx"
	"github.com/luannt2002/zalo-api-unofficial/internal/jsonx"
	"github.com/luannt2002/zalo-api-unofficial/model"
	"github.com/luannt2002/zalo-api-unofficial/pkg/format"
	"github.com/luannt2002/zalo-api-unofficial/session"
)

type (
	CreateBankAccountRequest struct {
		BinBank     model.BinBankCard `json:"binBank"`
		NumAccBank  string            `json:"numAccBank"`
		NameAccBank string            `json:"nameAccBank"`
	}
	CreateBankAccountResponse = model.BankAccount
	CreateBankAccountFn       = func(ctx context.Context, req CreateBankAccountRequest) (*CreateBankAccountResponse, error)
)

func (a *api) CreateBankAccount(ctx context.Context, req CreateBankAccountRequest) (*CreateBankAccountResponse, error) {
	return a.e.CreateBankAccount(ctx, req)
}

var createBankAccountFactory = apiFactory[*CreateBankAccountResponse, CreateBankAccountFn]()(
	func(a *api, sc session.Context, u factoryUtils[*CreateBankAccountResponse]) (CreateBankAccountFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("zimsg"), "")
		serviceURL := u.MakeURL(base+"/api/transfer/create", nil, true)

		return func(ctx context.Context, req CreateBankAccountRequest) (*CreateBankAccountResponse, error) {
			holderName := format.NormalizeHolderName(req.NameAccBank)
			if holderName == "" {
				return nil, errs.NewZCA("invalid account holder name: must be at least 5 alphanumeric characters", "api.CreateBankAccount")
			}

			payload := map[string]any{
				"bin":         int(req.BinBank),
				"bank_number": req.NumAccBank,
				"holder_name": holderName,
				"language":    sc.Language(),
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.CreateBankAccount", err)
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
