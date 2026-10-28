package events

import (
	"encoding/json"

	"github.com/luannt2002/zalo-api-unofficial/internal/jsonx"
	"github.com/luannt2002/zalo-api-unofficial/model"
)

type ReactionEventData struct {
	Reactions      []model.TReaction
	GroupReactions []model.TReaction
	// Errors lists items that could not be decoded; the others are kept.
	Errors []ItemError
}

func (m *ReactionEventData) UnmarshalJSON(data []byte) error {
	var raw struct {
		Reacts      json.RawMessage `json:"reacts"`
		ReactGroups json.RawMessage `json:"reactGroups"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	reactions, errs1, err := unmarshalReactions(raw.Reacts)
	if err != nil {
		return err
	}
	groupReactions, errs2, err := unmarshalReactions(raw.ReactGroups)
	if err != nil {
		return err
	}

	m.Reactions = reactions
	m.GroupReactions = groupReactions
	m.Errors = append(errs1, errs2...)
	return nil
}

func unmarshalReactions(b json.RawMessage) ([]model.TReaction, []ItemError, error) {
	if len(b) == 0 || string(b) == "null" {
		return nil, nil, nil
	}

	var items []json.RawMessage
	if len(b) > 0 && b[0] == '[' {
		if err := json.Unmarshal(b, &items); err != nil {
			return nil, nil, err
		}
	} else {
		items = []json.RawMessage{b}
	}

	type alias model.TReaction

	out := make([]model.TReaction, 0, len(items))
	var bad []ItemError
	for _, it := range items {
		var r model.TReaction
		aux := &struct {
			Content string `json:"content"`
			*alias
		}{
			alias: (*alias)(&r),
		}

		if err := jsonx.UnmarshalLenient(it, aux); err != nil {
			bad = append(bad, newItemError(it, err))
			continue
		}

		if aux.Content == "" {
			r.Content = model.ReactionContent{}
		} else {
			var content model.ReactionContent
			if err := jsonx.UnmarshalLenient([]byte(aux.Content), &content); err != nil {
				bad = append(bad, newItemError(it, err))
				continue
			}

			r.Content = content
		}

		out = append(out, r)
	}
	return out, bad, nil
}
