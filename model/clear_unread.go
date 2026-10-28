package model

import "encoding/json"

// ClearUnread: a conversation was read on another device of the same account
// (cmd 504 for 1-1, cmd 524 for groups; upstream PR #380).
type ClearUnread struct {
	Type      ThreadType      // ThreadTypeUser (504) or ThreadTypeGroup (524)
	ThreadID  string          // idTo
	LastMsgID string          // "" when the frame carries none
	Data      json.RawMessage // the raw clearUnreads item
}
