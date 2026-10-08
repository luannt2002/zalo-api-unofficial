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
	GetReminderResponse = model.ReminderGroup
	GetReminderFn       = func(ctx context.Context, reminderID string) (*GetReminderResponse, error)
)

func (a *api) GetReminder(ctx context.Context, reminderID string) (*GetReminderResponse, error) {
	return a.e.GetReminder(ctx, reminderID)
}

var getReminderFactory = apiFactory[*GetReminderResponse, GetReminderFn]()(
	func(a *api, sc session.Context, u factoryUtils[*GetReminderResponse]) (GetReminderFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("group_board"), "")
		serviceURL := u.MakeURL(base+"/api/board/topic/getReminder", nil, true)

		return func(ctx context.Context, reminderID string) (*GetReminderResponse, error) {
			payload := map[string]any{
				"eventId": reminderID,
				"imei":    sc.IMEI(),
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.GetReminder", err)
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
