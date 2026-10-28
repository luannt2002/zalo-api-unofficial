package events

import (
	"encoding/json"
	"fmt"

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

	// Err is set when the typed payload of a known act_type could not be decoded.
	Err *ItemError `json:"-"`

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
	if err := jsonx.UnmarshalLenient(data, &raw); err != nil {
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
	var err error
	switch c.ActionType {
	case "file_done":
		var ul uploadFileInfo
		if err = jsonx.UnmarshalLenient(payload, &ul); err == nil && ul.URL != "" {
			cd.UploadAttachment = &ul
		}
	case "group":
		cd.GroupEvent, err = decodeGroupEvent(raw.Action, payload)
	case "fr":
		cd.FriendEvent, err = decodeFriendEvent(raw.Action, payload)
	}
	if err != nil {
		// Keep the other controls of the frame; the listener reports this one.
		ie := newItemError(payload, fmt.Errorf("%s/%s: %w", c.ActionType, c.Action, err))
		c.Err = &ie
	}

	c.Data = cd
	return nil
}

func decodeGroupEvent(action string, data []byte) (model.TGroupEvent, error) {
	switch model.ParseGroupEventType(action) {
	case model.GroupEventTypeJoinRequest:
		var ev model.TGroupEventJoinRequest
		if err := jsonx.UnmarshalLenient(data, &ev); err != nil {
			return nil, err
		}
		return ev, nil

	case model.GroupEventTypeNewPinTopic,
		model.GroupEventTypeUnpinTopic,
		model.GroupEventTypeUpdatePinTopic:
		var ev model.TGroupEventPinTopic
		if err := jsonx.UnmarshalLenient(data, &ev); err != nil {
			return nil, err
		}
		return ev, nil

	case model.GroupEventTypeReorderPinTopic:
		var ev model.TGroupEventReorderPinTopic
		if err := jsonx.UnmarshalLenient(data, &ev); err != nil {
			return nil, err
		}
		return ev, nil

	case model.GroupEventTypeUpdateBoard,
		model.GroupEventTypeRemoveBoard:
		var ev model.TGroupEventBoard
		if err := jsonx.UnmarshalLenient(data, &ev); err != nil {
			return nil, err
		}
		return ev, nil

	case model.GroupEventTypeAcceptRemind,
		model.GroupEventTypeRejectRemind:
		var ev model.TGroupEventRemindRespond
		if err := jsonx.UnmarshalLenient(data, &ev); err != nil {
			return nil, err
		}
		return ev, nil

	case model.GroupEventTypeRemindTopic:
		var ev model.TGroupEventRemindTopic
		if err := jsonx.UnmarshalLenient(data, &ev); err != nil {
			return nil, err
		}
		return ev, nil

	default:
		var ev model.TGroupEventBase
		if err := jsonx.UnmarshalLenient(data, &ev); err != nil {
			return nil, err
		}
		return ev, nil
	}
}

func decodeFriendEvent(action string, data []byte) (model.TFriendEvent, error) {
	switch model.ParseFriendEventType(action) {
	case model.FriendEventTypeRequest:
		var ev model.TFriendEventRequest
		if err := jsonx.UnmarshalLenient(data, &ev); err != nil {
			return nil, err
		}
		return ev, nil

	case model.FriendEventTypeRejectRequest,
		model.FriendEventTypeUndoRequest:
		var ev model.TFriendEventRejectUndo
		if err := jsonx.UnmarshalLenient(data, &ev); err != nil {
			return nil, err
		}
		return ev, nil

	case model.FriendEventTypeSeenFriendRequest:
		var ev model.TFriendEventSeenRequest
		if err := jsonx.UnmarshalLenient(data, &ev); err != nil {
			return nil, err
		}
		return ev, nil

	case model.FriendEventTypePinCreate:
		var ev model.TFriendEventPinCreate
		if err := jsonx.UnmarshalLenient(data, &ev); err != nil {
			return nil, err
		}
		return ev, nil

	case model.FriendEventTypePinUnpin:
		var ev model.TFriendEventPinUnpin
		if err := jsonx.UnmarshalLenient(data, &ev); err != nil {
			return nil, err
		}
		return ev, nil

	case model.FriendEventTypeAdd,
		model.FriendEventTypeRemove,
		model.FriendEventTypeBlock,
		model.FriendEventTypeUnblock,
		model.FriendEventTypeBlockCall,
		model.FriendEventTypeUnblockCall:
		ev := model.TFriendEventBase(data)
		return ev, nil

	default:
		return nil, nil // no decoder: the listener exposes it as unhandled
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
