package events

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/luannt2002/zalo-api-unofficial/internal/jsonx"
	"github.com/luannt2002/zalo-api-unofficial/model"
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

// isUndo peeks msgType, so ordinary messages skip a failing TUndo decode
// (which takes the slow lenient path on every text message).
func isUndo(data []byte) bool {
	var h struct {
		MsgType string `json:"msgType"`
	}
	_ = json.Unmarshal(data, &h)
	return h.MsgType == "chat.undo"
}

// decodeUndo decodes a chat.undo item. A recall that does not decode is an
// error: falling back to the message path would show it as a new message.
func decodeUndo(data []byte) (*model.TUndo, *ItemError) {
	var tu model.TUndo
	err := jsonx.UnmarshalLenient(data, &tu)
	if err == nil && tu.MsgID != "" {
		return &tu, nil
	}
	if err == nil {
		err = errNoMsgID
	}
	ie := newItemError(data, fmt.Errorf("undo: %w", err))
	return nil, &ie
}
