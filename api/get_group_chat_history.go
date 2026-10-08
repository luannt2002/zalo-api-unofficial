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
	getGroupChatHistoryRaw struct {
		LastActionID      string                `json:"lastActionId"`
		LastActionIDOther string                `json:"lastActionIdOther"`
		More              int                   `json:"more"`
		GroupMsgs         []model.TGroupMessage `json:"groupMsgs"`
	}

	GetGroupChatHistoryResponse struct {
		LastActionID      string               `json:"lastActionId"`
		LastActionIDOther string               `json:"lastActionIdOther"`
		More              int                  `json:"more"`
		GroupMsgs         []model.GroupMessage `json:"groupMsgs"`
	}
	GetGroupChatHistoryFn = func(ctx context.Context, groupID string, count int) (*GetGroupChatHistoryResponse, error)
)

func (a *api) GetGroupChatHistory(ctx context.Context, groupID string, count int) (*GetGroupChatHistoryResponse, error) {
	return a.e.GetGroupChatHistory(ctx, groupID, count)
}

var getGroupChatHistoryFactory = apiFactory[*getGroupChatHistoryRaw, GetGroupChatHistoryFn]()(
	func(a *api, sc session.Context, u factoryUtils[*getGroupChatHistoryRaw]) (GetGroupChatHistoryFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("group"), "")
		serviceURL := u.MakeURL(base+"/api/group/history", nil, true)

		return func(ctx context.Context, groupID string, count int) (*GetGroupChatHistoryResponse, error) {
			if count <= 0 {
				count = 50
			}

			payload := map[string]any{
				"grid":  groupID,
				"count": count,
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.GetGroupChatHistory", err)
			}

			url := u.MakeURL(serviceURL, map[string]any{"params": enc}, true)
			resp, err := u.Request(ctx, url, &httpx.RequestOptions{Method: http.MethodGet})
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()

			raw, err := u.Resolve(resp, true)
			if err != nil {
				return nil, err
			}

			out := &GetGroupChatHistoryResponse{
				LastActionID:      raw.LastActionID,
				LastActionIDOther: raw.LastActionIDOther,
				More:              raw.More,
				GroupMsgs:         make([]model.GroupMessage, 0, len(raw.GroupMsgs)),
			}
			for _, msg := range raw.GroupMsgs {
				out.GroupMsgs = append(out.GroupMsgs, model.NewGroupMessage(sc.UID(), msg))
			}

			return out, nil
		}, nil
	},
)
