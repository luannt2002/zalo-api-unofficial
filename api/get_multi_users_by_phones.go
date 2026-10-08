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

var ErrMissingPhoneNumbers = errs.NewZCA("missing phoneNumbers", "api.GetMultiUsersByPhones")

type (
	GetMultiUsersByPhonesResponse map[string]UserBasic
	GetMultiUsersByPhonesFn       = func(ctx context.Context, phoneNumbers []string, avatarSize ...AvatarSize) (*GetMultiUsersByPhonesResponse, error)
)

func (a *api) GetMultiUsersByPhones(ctx context.Context, phoneNumbers []string, avatarSize ...AvatarSize) (*GetMultiUsersByPhonesResponse, error) {
	return a.e.GetMultiUsersByPhones(ctx, phoneNumbers, avatarSize...)
}

var getMultiUsersByPhonesFactory = apiFactory[*GetMultiUsersByPhonesResponse, GetMultiUsersByPhonesFn]()(
	func(a *api, sc session.Context, u factoryUtils[*GetMultiUsersByPhonesResponse]) (GetMultiUsersByPhonesFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("friend"), "")
		serviceURL := u.MakeURL(base+"/api/friend/profile/multiget", nil, true)

		return func(ctx context.Context, phoneNumbers []string, avatarSize ...AvatarSize) (*GetMultiUsersByPhonesResponse, error) {
			if len(phoneNumbers) == 0 {
				return nil, ErrMissingPhoneNumbers
			}

			size := AvatarSizeLarge
			if len(avatarSize) > 0 {
				size = avatarSize[0]
			}

			phones := make([]string, len(phoneNumbers))
			for i, phone := range phoneNumbers {
				if strings.HasPrefix(phone, "0") && sc.Language() == "vi" {
					phone = "84" + phone[1:]
				}
				phones[i] = phone
			}

			payload := map[string]any{
				"phones":      phones,
				"avatar_size": int(size),
				"language":    sc.Language(),
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.GetMultiUsersByPhones", err)
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
