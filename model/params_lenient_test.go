package model

import (
	"encoding/json"
	"testing"
)

// Tham số ghim tin trong sự kiện nhóm/bạn bè: id tin có thể tới dạng số hoặc chuỗi (kể cả bọc
// trong chuỗi JSON). Giải cứng thì cả sự kiện nhóm bị bỏ.
func TestPinParamsChiuChuoiLanSo(t *testing.T) {
	raw := `"{\"senderUid\":200,\"client_msg_id\":1761624700000,\"global_msg_id\":7300000000001,\"msg_type\":\"1\",\"title\":\"t\"}"`

	var p PinnedMessageDetailParams
	if err := json.Unmarshal([]byte(raw), &p); err != nil || p.ClientMsgID != "1761624700000" || p.GlobalMsgID != "7300000000001" || p.SenderUID != "200" || p.MsgType != 1 {
		t.Fatalf("PinnedMessageDetailParams = %+v, err %v", p, err)
	}
	var f TFriendEventPinCreateTopicParams
	if err := json.Unmarshal([]byte(raw), &f); err != nil || f.ClientMsgID != "1761624700000" || f.GlobalMsgID != "7300000000001" {
		t.Fatalf("TFriendEventPinCreateTopicParams = %+v, err %v", f, err)
	}
	var g GroupTopicFileMessageParams
	if err := json.Unmarshal([]byte(`{"senderUid":200,"client_msg_id":1,"global_msg_id":2,"msg_type":"1","extra":""}`), &g); err != nil || g.ClientMsgID != "1" || g.GlobalMsgID != "2" {
		t.Fatalf("GroupTopicFileMessageParams = %+v, err %v", g, err)
	}
}
