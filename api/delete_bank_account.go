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
	"github.com/luannt2002/zalo-api-unofficial/session"
)

type (
	DeleteBankAccountResponse struct {
		HasMore bool                `json:"hasMore"`
		Total   int                 `json:"total"`
		MyBanks []model.BankAccount `json:"myBanks"`
	}
	DeleteBankAccountFn = func(ctx context.Context, accountID int64, isDefault bool) (*DeleteBankAccountResponse, error)
)

func (a *api) DeleteBankAccount(ctx context.Context, accountID int64, isDefault bool) (*DeleteBankAccountResponse, error) {
	return a.e.DeleteBankAccount(ctx, accountID, isDefault)
}

var deleteBankAccountFactory = apiFactory[*DeleteBankAccountResponse, DeleteBankAccountFn]()(
	func(a *api, sc session.Context, u factoryUtils[*DeleteBankAccountResponse]) (DeleteBankAccountFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("zimsg"), "")
		serviceURL := u.MakeURL(base+"/api/transfer/delete", nil, true)

		return func(ctx context.Context, accountID int64, isDefault bool) (*DeleteBankAccountResponse, error) {
			payload := map[string]any{
				"account_id": accountID,
				"is_default": isDefault,
				"language":   sc.Language(),
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.DeleteBankAccount", err)
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
