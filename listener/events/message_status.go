package events

import (
	"github.com/luannt2002/zalo-api-unofficial/model"
)

type MessageStatusEventData struct {
	DeliveredMessages []model.TDeliveredMessage `json:"delivereds"`
	SeenMessages      []model.TUserSeenMessage  `json:"seens"`
}
