package api

import (
	"encoding/json"
	"testing"

	"github.com/luannt2002/zalo-api-unofficial/model"
)

func TestGetGroupChatHistoryUnmarshal(t *testing.T) {
	// 1. Nested string JSON payload (Zalo cm/* characteristic behavior)
	stringifiedPayload := `"{ \"lastMsgId\": 1804321268, \"hasMore\": true, \"groupMsgs\": [] }"`

	var raw getGroupChatHistoryRaw
	if err := json.Unmarshal([]byte(stringifiedPayload), &raw); err != nil {
		t.Fatalf("unmarshal stringified payload failed: %v", err)
	}

	if raw.LastMsgID.String() != "1804321268" {
		t.Fatalf("expected lastMsgId 1804321268, got: %s", raw.LastMsgID.String())
	}
	if !raw.HasMore {
		t.Fatalf("expected hasMore true, got false")
	}

	// 2. Direct JSON object
	directPayload := `{ "lastMsgId": 9999999, "hasMore": false, "groupMsgs": [] }`
	var direct getGroupChatHistoryRaw
	if err := json.Unmarshal([]byte(directPayload), &direct); err != nil {
		t.Fatalf("unmarshal direct payload failed: %v", err)
	}
	if direct.LastMsgID.String() != "9999999" {
		t.Fatalf("expected lastMsgId 9999999, got: %s", direct.LastMsgID.String())
	}
}

func TestFilterDeletedMessages(t *testing.T) {
	// Fixture từ filter-probe.mjs (Wave 25 _checkDeletedMessage)
	msgs := []model.GroupMessage{
		{Data: model.TGroupMessage{TMessage: model.TMessage{MsgID: "9"}}},
		{Data: model.TGroupMessage{TMessage: model.TMessage{MsgID: "10"}}},
		{Data: model.TGroupMessage{TMessage: model.TMessage{MsgID: "11"}}},
	}

	// Case 1: lastDeletedMsgID == "0" -> không xoá tin nào
	filtered, hasMore, lastMsgID := FilterDeletedMessages(msgs, "0", true, "9")
	if len(filtered) != 3 {
		t.Fatalf("expected 3 msgs, got %d", len(filtered))
	}
	if !hasMore {
		t.Fatalf("expected hasMore true")
	}
	if lastMsgID != "9" {
		t.Fatalf("expected lastMsgId 9, got %s", lastMsgID)
	}

	// Case 2: lastDeletedMsgID == "10" -> xoá msg 9 và 10, giữ msg 11, hasMore=false, lastMsgID="10"
	filtered, hasMore, lastMsgID = FilterDeletedMessages(msgs, "10", true, "9")
	if len(filtered) != 1 {
		t.Fatalf("expected 1 msg, got %d", len(filtered))
	}
	if filtered[0].Data.MsgID != "11" {
		t.Fatalf("expected remaining msg ID 11, got %s", filtered[0].Data.MsgID)
	}
	if hasMore {
		t.Fatalf("expected hasMore false after drop")
	}
	if lastMsgID != "10" {
		t.Fatalf("expected lastMsgId 10, got %s", lastMsgID)
	}

	// Case 3: ID lớn hơn Number safe integer (> 2^53)
	largeMsgs := []model.GroupMessage{{Data: model.TGroupMessage{TMessage: model.TMessage{MsgID: "9007199254740993"}}}}
	filtered, hasMore, lastMsgID = FilterDeletedMessages(largeMsgs, "9007199254740992", true, "0")
	if len(filtered) != 1 {
		t.Fatalf("expected 1 large msg kept, got %d", len(filtered))
	}
	if !hasMore {
		t.Fatalf("expected hasMore true")
	}
	if lastMsgID != "0" {
		t.Fatalf("expected lastMsgId 0, got %s", lastMsgID)
	}
}

func TestMergeCloudBoundary(t *testing.T) {
	// 1. Conversation mismatch
	_, err := MergeCloudBoundary(
		CloudBoundary{ConversationID: "conv1", HasMore: 1, CloudFirstSmsLocalID: "100"},
		CloudBoundary{ConversationID: "conv2", HasMore: 1, CloudFirstSmsLocalID: "100"},
	)
	if err == nil {
		t.Fatalf("expected error on conversation mismatch")
	}

	// 2. hasMore=0 dominates hasMore=1
	res, err := MergeCloudBoundary(
		CloudBoundary{ConversationID: "conv1", HasMore: 0, CloudFirstSmsLocalID: "50"},
		CloudBoundary{ConversationID: "conv1", HasMore: 1, CloudFirstSmsLocalID: "100"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.HasMore != 0 || res.CloudFirstSmsLocalID != "50" {
		t.Fatalf("expected hasMore=0, first=50; got hasMore=%d, first=%s", res.HasMore, res.CloudFirstSmsLocalID)
	}

	// 3. Both hasMore=1, selects min non-zero
	res, err = MergeCloudBoundary(
		CloudBoundary{ConversationID: "conv1", HasMore: 1, CloudFirstSmsLocalID: "100"},
		CloudBoundary{ConversationID: "conv1", HasMore: 1, CloudFirstSmsLocalID: "50"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.HasMore != 1 || res.CloudFirstSmsLocalID != "50" {
		t.Fatalf("expected hasMore=1, first=50; got hasMore=%d, first=%s", res.HasMore, res.CloudFirstSmsLocalID)
	}

	// 4. Large ID precision (> 2^53)
	res, err = MergeCloudBoundary(
		CloudBoundary{ConversationID: "conv1", HasMore: 1, CloudFirstSmsLocalID: "9007199254740993"},
		CloudBoundary{ConversationID: "conv1", HasMore: 1, CloudFirstSmsLocalID: "9007199254740995"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.CloudFirstSmsLocalID != "9007199254740993" {
		t.Fatalf("expected exact BigInt 9007199254740993, got %s", res.CloudFirstSmsLocalID)
	}
}
