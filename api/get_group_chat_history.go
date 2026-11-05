package api

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"

	"github.com/luannt2002/zalo-api-unofficial/errs"
	"github.com/luannt2002/zalo-api-unofficial/internal/httpx"
	"github.com/luannt2002/zalo-api-unofficial/internal/jsonx"
	"github.com/luannt2002/zalo-api-unofficial/model"
	"github.com/luannt2002/zalo-api-unofficial/session"
)

type (
	getGroupChatHistoryRaw struct {
		Error             int                   `json:"error"`
		LastMsgID         json.Number           `json:"lastMsgId"`
		MinMsgID          json.Number           `json:"minMsgId"`
		MaxMsgID          json.Number           `json:"maxMsgId"`
		MsgJumpID         json.Number           `json:"msgJumpId"`
		HasMore           bool                  `json:"hasMore"`
		IsOld             bool                  `json:"isOld"`
		GroupMsgs         []model.TGroupMessage `json:"groupMsgs"`
		LastActionID      string                `json:"lastActionId"`
		LastActionIDOther string                `json:"lastActionIdOther"`
		More              int                   `json:"more"`
	}

	GetGroupChatHistoryRequest struct {
		GroupID          string   `json:"groupId"`
		Count            int      `json:"count"`
		Cursor           string   `json:"cursor"`           // globalMsgId (decimal string, e.g. "0")
		Direction        string   `json:"direction"`        // "top" (default), "bottom", "jump"
		IsOld            bool     `json:"isOld"`            // for direction="top": false -> /api/cm/getrecentv2 (12506), true -> /api/cm/getoldv2 (12507)
		LocalMsgIDs      []string `json:"localMsgIds"`
		IsOA             bool     `json:"isOA"`
		LastDeletedMsgID string   `json:"lastDeletedMsgId"` // optional: if set, filters out msgs where msgId <= lastDeletedMsgID
	}

	GetGroupChatHistoryResponse struct {
		LastMsgID         string               `json:"lastMsgId"`
		MinMsgID          string               `json:"minMsgId"`
		MaxMsgID          string               `json:"maxMsgId"`
		HasMore           bool                 `json:"hasMore"`
		GroupMsgs         []model.GroupMessage `json:"groupMsgs"`
		LastActionID      string               `json:"lastActionId"`
		LastActionIDOther string               `json:"lastActionIdOther"`
		More              int                  `json:"more"`
	}

	CloudBoundary struct {
		ConversationID       string `json:"conversationId"`
		HasMore              int    `json:"hasMore"`              // 0 or 1
		CloudFirstSmsLocalID string `json:"cloudFirstSmsLocalId"` // decimal string (BigInt safe)
	}

	GetGroupChatHistoryFn   = func(ctx context.Context, groupID string, count int) (*GetGroupChatHistoryResponse, error)
	GetGroupChatHistoryExFn = func(ctx context.Context, req GetGroupChatHistoryRequest) (*GetGroupChatHistoryResponse, error)
)

func (r *getGroupChatHistoryRaw) UnmarshalJSON(b []byte) error {
	// Endpoint /api/cm/* có thể trả về data là chuỗi JSON lồng cần parse lớp 2
	var rawStr string
	if err := jsonx.UnmarshalLenient(b, &rawStr); err == nil && rawStr != "" && strings.HasPrefix(rawStr, "{") {
		b = []byte(rawStr)
	}

	type Alias getGroupChatHistoryRaw
	var a Alias
	if err := jsonx.UnmarshalLenient(b, &a); err != nil {
		return err
	}
	*r = getGroupChatHistoryRaw(a)
	return nil
}

// FilterDeletedMessages implements _checkDeletedMessage from web bundle (Wave 25).
// Drops messages where msgId <= lastDeletedMsgID. When dropped, sets hasMore = false
// and sets lastMsgID = lastDeletedMsgID.
func FilterDeletedMessages(msgs []model.GroupMessage, lastDeletedMsgID string, hasMore bool, lastMsgID string) ([]model.GroupMessage, bool, string) {
	if lastDeletedMsgID == "" || lastDeletedMsgID == "0" {
		return msgs, hasMore, lastMsgID
	}
	delThreshold := new(big.Int)
	if _, ok := delThreshold.SetString(lastDeletedMsgID, 10); !ok {
		return msgs, hasMore, lastMsgID
	}

	filtered := make([]model.GroupMessage, 0, len(msgs))
	dropped := false
	for _, m := range msgs {
		msgID := new(big.Int)
		if _, ok := msgID.SetString(m.Data.MsgID, 10); ok {
			if msgID.Cmp(delThreshold) <= 0 {
				dropped = true
				continue
			}
		}
		filtered = append(filtered, m)
	}

	if dropped {
		hasMore = false
		lastMsgID = lastDeletedMsgID
	}
	return filtered, hasMore, lastMsgID
}

// MergeCloudBoundary merges boundary state between segments with BigInt precision
// matching mergeCloudBoundary from cloud-history-contract.mjs.
func MergeCloudBoundary(a, b CloudBoundary) (*CloudBoundary, error) {
	if a.ConversationID != b.ConversationID {
		return nil, fmt.Errorf("conversation mismatch: %s vs %s", a.ConversationID, b.ConversationID)
	}
	if (a.HasMore != 0 && a.HasMore != 1) || (b.HasMore != 0 && b.HasMore != 1) {
		return nil, fmt.Errorf("invalid hasMore value, must be 0 or 1")
	}
	aa := new(big.Int)
	if _, ok := aa.SetString(a.CloudFirstSmsLocalID, 10); !ok {
		return nil, fmt.Errorf("invalid cloudFirstSmsLocalId for a: %s", a.CloudFirstSmsLocalID)
	}
	bb := new(big.Int)
	if _, ok := bb.SetString(b.CloudFirstSmsLocalID, 10); !ok {
		return nil, fmt.Errorf("invalid cloudFirstSmsLocalId for b: %s", b.CloudFirstSmsLocalID)
	}

	var first *big.Int
	zero := big.NewInt(0)

	if a.HasMore != b.HasMore {
		if a.HasMore == 0 {
			first = aa
		} else {
			first = bb
		}
	} else if a.HasMore == 1 {
		if aa.Cmp(zero) == 0 {
			first = bb
		} else if bb.Cmp(zero) == 0 {
			first = aa
		} else if aa.Cmp(bb) < 0 {
			first = aa
		} else {
			first = bb
		}
	} else {
		// both hasMore == 0
		if aa.Cmp(bb) > 0 {
			first = aa
		} else {
			first = bb
		}
	}

	hasMore := 1
	if a.HasMore == 0 || b.HasMore == 0 {
		hasMore = 0
	}

	return &CloudBoundary{
		ConversationID:       a.ConversationID,
		HasMore:              hasMore,
		CloudFirstSmsLocalID: first.String(),
	}, nil
}

func (a *api) GetGroupChatHistory(ctx context.Context, groupID string, count int) (*GetGroupChatHistoryResponse, error) {
	return a.e.GetGroupChatHistory(ctx, groupID, count)
}

func (a *api) GetGroupChatHistoryEx(ctx context.Context, req GetGroupChatHistoryRequest) (*GetGroupChatHistoryResponse, error) {
	return a.e.GetGroupChatHistoryEx(ctx, req)
}

var getGroupChatHistoryFactory = apiFactory[*getGroupChatHistoryRaw, GetGroupChatHistoryFn]()(
	func(a *api, sc session.Context, u factoryUtils[*getGroupChatHistoryRaw]) (GetGroupChatHistoryFn, error) {
		return func(ctx context.Context, groupID string, count int) (*GetGroupChatHistoryResponse, error) {
			return a.GetGroupChatHistoryEx(ctx, GetGroupChatHistoryRequest{
				GroupID:   groupID,
				Count:     count,
				Cursor:    "0",
				Direction: "top",
				IsOld:     false,
			})
		}, nil
	},
)

var getGroupChatHistoryExFactory = apiFactory[*getGroupChatHistoryRaw, GetGroupChatHistoryExFn]()(
	func(a *api, sc session.Context, u factoryUtils[*getGroupChatHistoryRaw]) (GetGroupChatHistoryExFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("group_cloud_message"), jsonx.FirstOr(sc.GetZpwService("group"), ""))
		if base == "" {
			base = jsonx.FirstOr(sc.GetZpwService("chat"), "")
		}

		return func(ctx context.Context, req GetGroupChatHistoryRequest) (*GetGroupChatHistoryResponse, error) {
			count := req.Count
			if count <= 0 {
				count = 50
			}

			cursor := req.Cursor
			if cursor == "" {
				cursor = "0"
			}

			direction := strings.ToLower(req.Direction)
			if direction == "" {
				direction = "top"
			}

			cleanGroupID := strings.TrimPrefix(req.GroupID, "g")

			// Dispatch endpoint theo direction & isOld (Wave 25)
			var path string
			if direction == "jump" || direction == "bottom" {
				path = "/api/cm/rgetv2" // cmd 12508
			} else if req.IsOld {
				path = "/api/cm/getoldv2" // cmd 12507
			} else {
				path = "/api/cm/getrecentv2" // cmd 12506
			}

			serviceURL := u.MakeURL(base+path, nil, true)

			payload := map[string]any{
				"groupId":     cleanGroupID,
				"globalMsgId": cursor,
				"count":       count,
				"src":         3,
			}

			if direction == "jump" || direction == "bottom" {
				jumpVal := 0
				if direction == "jump" {
					jumpVal = 1
				}
				payload["msgJump"] = jumpVal
				if len(req.LocalMsgIDs) > 0 {
					payload["msgIds"] = req.LocalMsgIDs
				} else {
					payload["msgIds"] = []string{}
				}
			} else {
				// direction == "top"
				payload["imei"] = sc.IMEI()
				if len(req.LocalMsgIDs) > 0 {
					payload["msgIds"] = req.LocalMsgIDs
				} else {
					payload["msgIds"] = []string{}
				}
			}

			if req.IsOA {
				payload["isOA"] = 1
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.GetGroupChatHistoryEx", err)
			}

			url := u.MakeURL(serviceURL, map[string]any{"params": enc, "nretry": 0}, true)
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
				LastMsgID:         raw.LastMsgID.String(),
				MinMsgID:          raw.MinMsgID.String(),
				MaxMsgID:          raw.MaxMsgID.String(),
				HasMore:           raw.HasMore || raw.More > 0,
				LastActionID:      raw.LastActionID,
				LastActionIDOther: raw.LastActionIDOther,
				More:              raw.More,
				GroupMsgs:         make([]model.GroupMessage, 0, len(raw.GroupMsgs)),
			}
			for _, msg := range raw.GroupMsgs {
				out.GroupMsgs = append(out.GroupMsgs, model.NewGroupMessage(sc.UID(), msg))
			}

			// Áp dụng bộ lọc _checkDeletedMessage nếu có yêu cầu LastDeletedMsgID
			if req.LastDeletedMsgID != "" && req.LastDeletedMsgID != "0" {
				out.GroupMsgs, out.HasMore, out.LastMsgID = FilterDeletedMessages(
					out.GroupMsgs, req.LastDeletedMsgID, out.HasMore, out.LastMsgID,
				)
			}

			return out, nil
		}, nil
	},
)
