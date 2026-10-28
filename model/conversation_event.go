package model

import "encoding/json"

// PinConversationsUpdate: conversations pinned/unpinned in the sidebar from any
// device (cmd 601 act_type "pinned_convers"). Not the same as pinning a message
// inside a chat, which arrives as a friend/group pin event.
type PinConversationsUpdate struct {
	Tab    int64
	Action int64    // 1 = pin, 2 = unpin
	PinIDs []string // "u<uid>" for a user, "g<groupId>" for a group
	TS     int64
	Data   json.RawMessage // the decoded control data as received
}
