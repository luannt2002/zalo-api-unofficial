package events

import (
	"encoding/json"
	"errors"

	"github.com/luannt2002/zalo-api-unofficial/internal/jsonx"
)

// ItemError is one item of a batch that could not be decoded; the other items
// of the batch are kept.
type ItemError struct {
	MsgID string
	Raw   json.RawMessage
	Err   error
}

var errNoMsgID = errors.New("item has no msgId")

func newItemError(raw []byte, err error) ItemError {
	var id struct {
		MsgID json.RawMessage `json:"msgId"`
	}
	_ = json.Unmarshal(raw, &id)
	return ItemError{MsgID: jsonx.ScalarString(id.MsgID), Raw: append(json.RawMessage(nil), raw...), Err: err}
}
