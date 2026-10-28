package events

import (
	"encoding/json"

	"github.com/luannt2002/zalo-api-unofficial/internal/jsonx"
	"github.com/luannt2002/zalo-api-unofficial/model"
)

type OldMessagesEventData struct {
	Msgs      []model.TMessage `json:"msgs"`
	GroupMsgs []model.TMessage `json:"groupMsgs"`
	// Errors lists items that could not be decoded; the others are kept.
	Errors []ItemError `json:"-"`
}

func (d *OldMessagesEventData) UnmarshalJSON(data []byte) error {
	var raw struct {
		Msgs      []json.RawMessage `json:"msgs"`
		GroupMsgs []json.RawMessage `json:"groupMsgs"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	d.Msgs = d.decode(raw.Msgs)
	d.GroupMsgs = d.decode(raw.GroupMsgs)
	return nil
}

func (d *OldMessagesEventData) decode(items []json.RawMessage) []model.TMessage {
	out := make([]model.TMessage, 0, len(items))
	for _, it := range items {
		var m model.TMessage
		if err := jsonx.UnmarshalLenient(it, &m); err != nil {
			d.Errors = append(d.Errors, newItemError(it, err))
			continue
		}
		out = append(out, m)
	}
	return out
}
