package events

import (
	"encoding/json"

	"github.com/luannt2002/zalo-api-unofficial/internal/jsonx"
	"github.com/luannt2002/zalo-api-unofficial/model"
)

type ControlEventData struct {
	Controls []ControlItem `json:"controls"`
}

type ControlItem struct {
	Content ControlContent `json:"content"`
}

type ControlContent struct {
	ActionType string      `json:"act_type"`
	Action     string      `json:"act"`
	Data       controlData `json:"data"`

	FileID *int64 `json:"fileId,omitempty"`

	// RawData is the control data with a string-encoded JSON layer removed, for
	// act_types that have no typed decoder (pinned_convers, voip).
	RawData json.RawMessage `json:"-"`
	// Content is the nested content.content field (mark_unread), string layer removed.
	Content json.RawMessage `json:"-"`
}

type controlData struct {
	UploadAttachment *uploadFileInfo
	GroupEvent       model.TGroupEvent
	FriendEvent      model.TFriendEvent
}

type uploadFileInfo struct {
	URL string `json:"url"`
}

func (d *ControlEventData) UnmarshalJSON(data []byte) error {
	type alias ControlEventData
	var tmp alias

	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}

	*d = ControlEventData(tmp)
	return nil
}

func (c *ControlContent) UnmarshalJSON(data []byte) error {
	var raw struct {
		ActionType string          `json:"act_type"`
		Action     string          `json:"act"`
		Data       json.RawMessage `json:"data"`
		Content    json.RawMessage `json:"content"`
		FileID     *int64          `json:"fileId,omitempty"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	c.Content = unquoteJSON(raw.Content)

	c.ActionType = raw.ActionType
	c.Action = raw.Action
	c.FileID = raw.FileID

	payload := raw.Data
	if len(payload) > 0 && payload[0] == '"' {
		var s string
		if err := json.Unmarshal(payload, &s); err != nil {
			return err
		}
		payload = []byte(s)
	}
	c.RawData = unquoteJSON(raw.Data)

	var cd controlData
	switch c.ActionType {
	case "file_done":
		var ul uploadFileInfo
		if err := json.Unmarshal(payload, &ul); err == nil && ul.URL != "" {
			cd.UploadAttachment = &ul
			c.Data = cd
			return nil
		}
	case "group":
		if ev, ok := decodeGroupEvent(raw.Action, payload); ok {
			cd.GroupEvent = ev
			c.Data = cd
			return nil
		}
	case "fr":
		if ev, ok := decodeFriendEvent(raw.Action, payload); ok {
			cd.FriendEvent = ev
			c.Data = cd
			return nil
		}
	}

	c.Data = cd
	return nil
}

func decodeGroupEvent(action string, data []byte) (model.TGroupEvent, bool) {
	switch model.ParseGroupEventType(action) {
	case model.GroupEventTypeJoinRequest:
		var ev model.TGroupEventJoinRequest
		if err := jsonx.UnmarshalLenient(data, &ev); err != nil {
			return nil, false
		}
		return ev, true

	case model.GroupEventTypeNewPinTopic,
		model.GroupEventTypeUnpinTopic,
		model.GroupEventTypeUpdatePinTopic:
		var ev model.TGroupEventPinTopic
		if err := jsonx.UnmarshalLenient(data, &ev); err != nil {
			return nil, false
		}
		return ev, true

	case model.GroupEventTypeReorderPinTopic:
		var ev model.TGroupEventReorderPinTopic
		if err := jsonx.UnmarshalLenient(data, &ev); err != nil {
			return nil, false
		}
		return ev, true

	case model.GroupEventTypeUpdateBoard,
		model.GroupEventTypeRemoveBoard:
		var ev model.TGroupEventBoard
		if err := jsonx.UnmarshalLenient(data, &ev); err != nil {
			return nil, false
		}
		return ev, true

	case model.GroupEventTypeAcceptRemind,
		model.GroupEventTypeRejectRemind:
		var ev model.TGroupEventRemindRespond
		if err := jsonx.UnmarshalLenient(data, &ev); err != nil {
			return nil, false
		}
		return ev, true

	case model.GroupEventTypeRemindTopic:
		var ev model.TGroupEventRemindTopic
		if err := jsonx.UnmarshalLenient(data, &ev); err != nil {
			return nil, false
		}
		return ev, true

	default:
		var ev model.TGroupEventBase
		if err := jsonx.UnmarshalLenient(data, &ev); err != nil {
			return nil, false
		}
		return ev, true
	}
}

func decodeFriendEvent(action string, data []byte) (model.TFriendEvent, bool) {
	switch model.ParseFriendEventType(action) {
	case model.FriendEventTypeRequest:
		var ev model.TFriendEventRequest
		if err := jsonx.UnmarshalLenient(data, &ev); err != nil {
			return nil, false
		}
		return ev, true

	case model.FriendEventTypeRejectRequest,
		model.FriendEventTypeUndoRequest:
		var ev model.TFriendEventRejectUndo
		if err := jsonx.UnmarshalLenient(data, &ev); err != nil {
			return nil, false
		}
		return ev, true

	case model.FriendEventTypeSeenFriendRequest:
		var ev model.TFriendEventSeenRequest
		if err := jsonx.UnmarshalLenient(data, &ev); err != nil {
			return nil, false
		}
		return ev, true

	case model.FriendEventTypePinCreate:
		var ev model.TFriendEventPinCreate
		if err := jsonx.UnmarshalLenient(data, &ev); err != nil {
			return nil, false
		}
		return ev, true

	case model.FriendEventTypePinUnpin:
		var ev model.TFriendEventPinUnpin
		if err := jsonx.UnmarshalLenient(data, &ev); err != nil {
			return nil, false
		}
		return ev, true

	case model.FriendEventTypeAdd,
		model.FriendEventTypeRemove,
		model.FriendEventTypeBlock,
		model.FriendEventTypeUnblock,
		model.FriendEventTypeBlockCall,
		model.FriendEventTypeUnblockCall:
		ev := model.TFriendEventBase(data)
		return ev, true

	default:
		return nil, false
	}
}

// unquoteJSON removes one string-encoded JSON layer ("{...}" -> {...}); other
// values are returned unchanged.
func unquoteJSON(b json.RawMessage) json.RawMessage {
	var s string
	if len(b) > 0 && b[0] == '"' && json.Unmarshal(b, &s) == nil && json.Valid([]byte(s)) {
		return json.RawMessage(s)
	}
	return b
}
