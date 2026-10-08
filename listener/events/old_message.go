package events

import (
	"github.com/luannt2002/zalo-api-unofficial/model"
)

type OldMessagesEventData struct {
	Msgs      []model.TMessage `json:"msgs"`
	GroupMsgs []model.TMessage `json:"groupMsgs"`
}
