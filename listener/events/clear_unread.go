package events

import "encoding/json"

type ClearUnreadEventData struct {
	ClearUnreads []json.RawMessage `json:"clearUnreads"`
}
