package api

import (
	"context"
	"net/http"

	"github.com/luannt2002/zalo-api-unofficial/errs"
	"github.com/luannt2002/zalo-api-unofficial/internal/httpx"
	"github.com/luannt2002/zalo-api-unofficial/internal/jsonx"
	"github.com/luannt2002/zalo-api-unofficial/session"
)

type (
	GetReminderResponseResponse struct {
		RejectMember []string `json:"rejectMember"`
		AcceptMember []string `json:"acceptMember"`
	}
	GetReminderResponseFn = func(ctx context.Context, reminderID string) (*GetReminderResponseResponse, error)
)

func (a *api) GetReminderResponse(ctx context.Context, reminderID string) (*GetReminderResponseResponse, error) {
	return a.e.GetReminderResponse(ctx, reminderID)
}

var getReminderResponseFactory = apiFactory[*GetReminderResponseResponse, GetReminderResponseFn]()(
	func(a *api, sc session.Context, u factoryUtils[*GetReminderResponseResponse]) (GetReminderResponseFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("group_board"), "")
		serviceURL := u.MakeURL(base+"/api/board/topic/listResponseEvent", nil, true)

		return func(ctx context.Context, reminderID string) (*GetReminderResponseResponse, error) {
			payload := map[string]any{
				"eventId": reminderID,
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.GetReminderResponse", err)
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
