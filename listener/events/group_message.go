package events

import (
	"github.com/luannt2002/zalo-api-unofficial/internal/jsonx"
	"github.com/luannt2002/zalo-api-unofficial/model"
)

type GroupMessageEventData struct {
	GroupMsgs []groupMessageOrUndo `json:"groupMsgs"`
}

type groupMessageOrUndo struct {
	Message *model.TGroupMessage
	Undo    *model.TUndo
	// Err is set (and Message/Undo are nil) when the item could not be decoded.
	Err *ItemError
}

func (m *groupMessageOrUndo) UnmarshalJSON(data []byte) error {
	var tu model.TUndo
	if err := jsonx.UnmarshalLenient(data, &tu); err == nil && tu.MsgID != "" && tu.MsgType == "chat.undo" {
		m.Undo = &tu
		return nil
	}
	var tm model.TGroupMessage
	err := jsonx.UnmarshalLenient(data, &tm)
	if err == nil && tm.MsgID != "" {
		m.Message = &tm
		return nil
	}
	if err == nil {
		err = errNoMsgID
	}
	// Keep the batch decodable; the listener reports this item instead of dropping it silently.
	ie := newItemError(data, err)
	m.Err = &ie
	return nil
}
