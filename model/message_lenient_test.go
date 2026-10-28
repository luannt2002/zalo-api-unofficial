package model

import (
	"encoding/json"
	"testing"

	"github.com/luannt2002/zalo-api-unofficial/internal/jsonx"
)

// Zalo gửi cùng một trường lúc là chuỗi, lúc là số. zca-js (JS) không để ý; bản Go giải mã
// cứng thì hỏng CẢ tin. Upstream chuẩn hoá rõ ràng quote.ownerId = String(ownerId).
func TestTQuoteOwnerIDNhanChuoiVaSo(t *testing.T) {
	tests := []struct {
		name, raw, want string
	}{
		{"chuỗi", `{"ownerId":"201","cliMsgId":1,"ts":2}`, "201"},
		{"số", `{"ownerId":201,"cliMsgId":1,"ts":2}`, "201"},
		{"số 19 chữ số", `{"ownerId":8070450532247928833}`, "8070450532247928833"},
		{"chuỗi 19 chữ số", `{"ownerId":"8070450532247928833"}`, "8070450532247928833"},
		{"vắng", `{"msg":"x"}`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var q TQuote
			if err := json.Unmarshal([]byte(tt.raw), &q); err != nil {
				t.Fatalf("Unmarshal(%s) lỗi: %v", tt.raw, err)
			}
			if q.OwnerID != tt.want {
				t.Fatalf("OwnerID = %q, mong %q", q.OwnerID, tt.want)
			}
		})
	}
}

// Các trường số khác của quote cũng có thể tới dạng chuỗi.
func TestTQuoteTruongSoDangChuoi(t *testing.T) {
	var q TQuote
	raw := `{"ownerId":"201","cliMsgId":"1761624700000","globalMsgId":"7300000000001","cliMsgType":"1","ts":"1761624700000","ttl":"0","msg":"gốc"}`
	if err := json.Unmarshal([]byte(raw), &q); err != nil {
		t.Fatalf("Unmarshal lỗi: %v", err)
	}
	if q.CliMsgID != 1761624700000 || q.GlobalMsgID != 7300000000001 || q.CliMsgType != 1 || q.Timestamp != 1761624700000 || q.Msg != "gốc" {
		t.Fatalf("q = %+v", q)
	}
}

// Một trường lệch kiểu ở bất kỳ đâu trong tin (status, ttl, ts, mentions, propertyExt, paramsExt,
// deleted content) không được làm hỏng cả tin.
func TestTGroupMessageChiuChuoiLanSo(t *testing.T) {
	raw := `{
		"msgId":"g1","cliMsgId":123,"uidFrom":"200","idTo":900,"ts":1761624780000,
		"status":"1","ttl":"0","cmd":"521","st":"3","at":"5",
		"content":"hi",
		"propertyExt":{"color":"0","size":"0","type":"0","subType":"0","ext":"{}"},
		"paramsExt":{"countUnread":"1","containType":"0","platformType":"1"},
		"mentions":[{"uid":201,"pos":"0","len":"2","type":"0"}],
		"quote":{"ownerId":"201","cliMsgId":"1"}
	}`
	var m TGroupMessage
	if err := jsonx.UnmarshalLenient([]byte(raw), &m); err != nil {
		t.Fatalf("UnmarshalLenient lỗi: %v", err)
	}
	if m.MsgID != "g1" || m.CliMsgID != "123" || m.IDTo != "900" || m.TS != "1761624780000" ||
		m.Status != 1 || m.CMD != 521 || m.ParamsExt.CountUnread != 1 || m.PropertyExt == nil ||
		len(m.Mentions) != 1 || m.Mentions[0].UID != "201" || m.Mentions[0].Len != 2 ||
		m.Quote == nil || m.Quote.OwnerID != "201" {
		t.Fatalf("m = %+v", m)
	}
}

func TestContentDeletedChiuChuoiLanSo(t *testing.T) {
	var c Content
	raw := `[{"type":"1","actionType":"2","uidFrom":"200","uidTo":201,"clientDelMsgId":"5","globalDelMsgId":"6","destId":"7"}]`
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		t.Fatalf("Unmarshal lỗi: %v", err)
	}
	if len(c.DeletedContent) != 1 || c.DeletedContent[0].UIDFrom != 200 || c.DeletedContent[0].GlobalDelMsgId != 6 {
		t.Fatalf("DeletedContent = %+v (Other=%v)", c.DeletedContent, c.Other)
	}
}

func TestContentAttachmentChiuChuoiLanSo(t *testing.T) {
	var c Content
	if err := json.Unmarshal([]byte(`{"title":"a.jpg","href":"h","childnumber":"2"}`), &c); err != nil {
		t.Fatalf("Unmarshal lỗi: %v", err)
	}
	if c.Attachment == nil || c.Attachment.ChildNumber != 2 {
		t.Fatalf("Attachment = %+v (Other=%v), mong typed attachment", c.Attachment, c.Other)
	}
}
