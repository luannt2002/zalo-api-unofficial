package events

import (
	"encoding/json"
	"strings"

	"github.com/luannt2002/zalo-api-unofficial/internal/jsonx"
	"github.com/luannt2002/zalo-api-unofficial/model"
)

type ActionEventData struct {
	Actions []actionItem `json:"actions"`
}

type actionItem struct {
	ActionType string     `json:"act_type"`
	Action     string     `json:"act"`
	Data       actionData `json:"data"`
}

type actionData struct {
	Typing      model.TTyping
	GroupTyping model.TGroupTyping
	// Raw is the data with a string-encoded JSON layer removed.
	Raw json.RawMessage
	// Err is set when the data could not be decoded as typing.
	Err *ItemError
}

func (m *actionData) UnmarshalJSON(data []byte) error {
	m.Raw = unquoteJSON(append(json.RawMessage(nil), data...))
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil || s == "" {
			return err
		}

		// Typing data arrives as `"uid":"…","ts":"…"` without braces (upstream: JSON.parse(`{${data}}`)).
		if !strings.HasPrefix(s, "{") {
			s = "{" + s
		}
		if !strings.HasSuffix(s, "}") {
			s += "}"
		}

		data = []byte(s)
	}

	// TGroupTyping embeds TTyping, so one decode serves both typing and gtyping.
	var gt model.TGroupTyping
	if err := jsonx.UnmarshalLenient(data, &gt); err != nil {
		ie := newItemError(data, err)
		m.Err = &ie
		return nil
	}
	m.GroupTyping = gt
	m.Typing = gt.TTyping
	return nil
}
