package api

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/luannt2002/zalo-api-unofficial/errs"
	"github.com/luannt2002/zalo-api-unofficial/internal/httpx"
	"github.com/luannt2002/zalo-api-unofficial/internal/jsonx"
	"github.com/luannt2002/zalo-api-unofficial/model"
	"github.com/luannt2002/zalo-api-unofficial/pkg/format"
	"github.com/luannt2002/zalo-api-unofficial/session"
)

type (
	UpdateBankAccountRequest struct {
		AccountID   int64             `json:"accountId"`
		BinBank     model.BinBankCard `json:"binBank"`
		NumAccBank  string            `json:"numAccBank"`
		NameAccBank string            `json:"nameAccBank"`
	}
	UpdateBankAccountResponse = model.BankAccount
	UpdateBankAccountFn       = func(ctx context.Context, req UpdateBankAccountRequest) (*UpdateBankAccountResponse, error)
)

func (a *api) UpdateBankAccount(ctx context.Context, req UpdateBankAccountRequest) (*UpdateBankAccountResponse, error) {
	return a.e.UpdateBankAccount(ctx, req)
}

var updateBankAccountFactory = apiFactory[*UpdateBankAccountResponse, UpdateBankAccountFn]()(
	func(a *api, sc session.Context, u factoryUtils[*UpdateBankAccountResponse]) (UpdateBankAccountFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("zimsg"), "")
		serviceURL := u.MakeURL(base+"/api/transfer/update", nil, true)

		return func(ctx context.Context, req UpdateBankAccountRequest) (*UpdateBankAccountResponse, error) {
			holderName := format.NormalizeHolderName(req.NameAccBank)
			if holderName == "" {
				return nil, errs.NewZCA("invalid account holder name: must be at least 5 alphanumeric characters", "api.UpdateBankAccount")
			}

			payload := map[string]any{
				"account_id":  req.AccountID,
				"bin":         int(req.BinBank),
				"bank_number": req.NumAccBank,
				"holder_name": holderName,
				"language":    sc.Language(),
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.UpdateBankAccount", err)
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
