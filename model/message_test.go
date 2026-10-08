package model

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestContentAttachment_PreservesExtraFields: Zalo gửi titled attachment kèm field PHỤ
// (normalUrl/oriUrl/hd/fileSize) không có trong TAttachmentContent. Round-trip parse→marshal
// PHẢI giữ nguyên các field đó (trước đây bị mất — inbound M4).
func TestContentAttachment_PreservesExtraFields(t *testing.T) {
	raw := `{"title":"anh.jpg","href":"https://zalo/x","thumb":"https://zalo/t","normalUrl":"https://zalo/normal","oriUrl":"https://zalo/ori","fileSize":12345}`
	var c Content
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		t.Fatalf("unmarshal lỗi: %v", err)
	}
	if c.Attachment == nil || c.Attachment.Title != "anh.jpg" {
		t.Fatalf("typed access hỏng: %+v", c.Attachment)
	}
	out, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal lỗi: %v", err)
	}
	for _, field := range []string{"normalUrl", "oriUrl", "fileSize", "12345"} {
		if !strings.Contains(string(out), field) {
			t.Fatalf("MẤT field %q sau round-trip: %s", field, out)
		}
	}
}

// TestContentString_RoundTrip: nhánh string vẫn hoạt động.
func TestContentString_RoundTrip(t *testing.T) {
	var c Content
	if err := json.Unmarshal([]byte(`"xin chào"`), &c); err != nil {
		t.Fatalf("unmarshal string lỗi: %v", err)
	}
	if c.String == nil || *c.String != "xin chào" {
		t.Fatalf("string branch hỏng: %+v", c.String)
	}
	out, _ := json.Marshal(c)
	if string(out) != `"xin chào"` {
		t.Fatalf("marshal string sai: %s", out)
	}
}
