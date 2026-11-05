package api

import (
	"context"
	"testing"

	"github.com/luannt2002/zalo-api-unofficial/model"
)

func TestAddReactionInvalidThreadType(t *testing.T) {
	// Giả lập trực tiếp factory validator
	dest := AddReactionDestination{
		Type: model.ThreadType(99), // Thread type không hợp lệ
		Data: AddReactionData{MsgID: "123", CliMsgID: "456"},
	}

	reaction := model.ReactionData{
		RIcon:  "👍",
		RType:  1,
		Source: 6,
	}

	a := &api{
		e: endpoints{
			AddReaction: func(ctx context.Context, d AddReactionDestination, r model.ReactionData) (*AddReactionResponse, error) {
				if d.Type != model.ThreadTypeUser && d.Type != model.ThreadTypeGroup {
					return nil, ErrInvalidThreadType
				}
				return &AddReactionResponse{}, nil
			},
		},
	}

	_, err := a.AddReaction(context.Background(), dest, reaction)
	if err == nil {
		t.Fatalf("expected error for invalid thread type, got nil")
	}
}

var ErrInvalidThreadType = &mockErr{"thread type is invalid"}

type mockErr struct{ msg string }

func (m *mockErr) Error() string { return m.msg }

