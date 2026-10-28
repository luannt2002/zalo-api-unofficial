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

// UnreadMarkUpdate: conversations marked ("add") or unmarked ("remove") as
// unread on another device (cmd 601 act_type "mark_unread").
type UnreadMarkUpdate struct {
	Action   string // "add" or "remove"
	UserIDs  []string
	GroupIDs []string
}

// VoipEvent: call signaling (cmd 601 act_type "voip"). The SDK does not
// interpret the call protocol; Data is the control data as received, with a
// string-encoded JSON layer removed.
type VoipEvent struct {
	Act  string // e.g. "request", "ring_ring", "answer", "cancel", "end_call"
	Data json.RawMessage
}
