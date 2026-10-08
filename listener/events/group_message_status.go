package events

import "github.com/luannt2002/zalo-api-unofficial/model"

type GroupMessageStatusEventData struct {
	DeliveredMessages []model.TGroupDeliveredMessage `json:"delivereds"`
	SeenMessages      []model.TGroupSeenMessage      `json:"groupSeens"`
}
