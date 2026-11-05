package api

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/luannt2002/zalo-api-unofficial/internal/testenv"
)

func TestZNoiseUnmarshal(t *testing.T) {
	env := testenv.Load(t)

	// 1. Nested response {"error_code": 0, "data": {"fids": [...], "gids": [...]}}
	nestedJSON := fmt.Sprintf(`{
		"error_code": 0,
		"error_message": "Successful",
		"data": {
			"fids": [%d],
			"gids": [-1543210746009390142]
		}
	}`, env.NoiseA)

	var nestedResp GetZNoiseResponse
	if err := json.Unmarshal([]byte(nestedJSON), &nestedResp); err != nil {
		t.Fatalf("unmarshal nested response: %v", err)
	}

	if len(nestedResp.Data.FIDs) != 1 || nestedResp.Data.FIDs[0] != env.NoiseA {
		t.Fatalf("nested FIDs mismatch: got %v, want [%d]", nestedResp.Data.FIDs, env.NoiseA)
	}
	if len(nestedResp.Data.GIDs) != 1 || nestedResp.Data.GIDs[0] != -1543210746009390142 {
		t.Fatalf("nested GIDs mismatch: got %v", nestedResp.Data.GIDs)
	}

	// 2. Flat response {"fids": [...], "gids": [...]}
	flatJSON := fmt.Sprintf(`{
		"error_code": 0,
		"fids": [%d],
		"gids": [-1543210746009390142]
	}`, env.NoiseA)

	var flatResp GetZNoiseResponse
	if err := json.Unmarshal([]byte(flatJSON), &flatResp); err != nil {
		t.Fatalf("unmarshal flat response: %v", err)
	}

	if len(flatResp.Data.FIDs) != 1 || flatResp.Data.FIDs[0] != env.NoiseA {
		t.Fatalf("flat FIDs mismatch: got %v, want [%d]", flatResp.Data.FIDs, env.NoiseA)
	}

	// 3. Mathematical invariant: All znoise values must be negative int64 with bit 63 set
	if uint64(env.NoiseA)>>63 != 1 {
		t.Fatalf("expected bit 63 to be 1 for negative noise ID, got: 0x%x", env.NoiseA)
	}
}

