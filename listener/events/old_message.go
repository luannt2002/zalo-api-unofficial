package events

// OldMessagesEventData is cmd 510 (msgs) / 511 (groupMsgs). Items are decoded
// one by one like live messages: a replayed recall is an Undo (upstream PR #391)
// and an undecodable item sets Err.
type OldMessagesEventData struct {
	Msgs      []messageOrUndo      `json:"msgs"`
	GroupMsgs []groupMessageOrUndo `json:"groupMsgs"`
}
