package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/luannt2002/zalo-api-unofficial/internal/cryptox"
	"github.com/luannt2002/zalo-api-unofficial/internal/testenv"
	"github.com/luannt2002/zalo-api-unofficial/session"
)

func TestMiniProfileTypeEnum(t *testing.T) {
	jsonPayload := `{
		"user1": {
			"userId": "user1",
			"type": 0
		},
		"oa1": {
			"userId": "oa1",
			"type": 5
		},
		"zoa1": {
			"userId": "zoa1",
			"type": 7
		}
	}`

	var resp GetMiniProfilesResponse
	if err := json.Unmarshal([]byte(jsonPayload), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp["user1"].Type != MiniProfileTypeUser {
		t.Fatalf("expected MiniProfileTypeUser (0), got: %d", resp["user1"].Type)
	}
	if resp["oa1"].Type != MiniProfileTypeOA {
		t.Fatalf("expected MiniProfileTypeOA (5), got: %d", resp["oa1"].Type)
	}
	if resp["zoa1"].Type != MiniProfileTypeZOA {
		t.Fatalf("expected MiniProfileTypeZOA (7), got: %d", resp["zoa1"].Type)
	}
}

func TestGetMiniProfilesValidation(t *testing.T) {
	env := testenv.Load(t)
	sc := session.NewContext()
	sc.SealLogin(session.Seal{
		UID:       env.UIDA,
		UserAgent: "test-user-agent",
		SecretKey: session.SecretKey("dGVzdC1zZWNyZXQta2V5LTE2Yg=="), // 16 bytes base64
		Settings:  &session.Settings{},
	})

	fn, err := getMiniProfilesFactory(sc, &api{sc: sc})
	if err != nil {
		t.Fatalf("factory failed: %v", err)
	}

	ctx := context.Background()

	// 1. Empty slice
	if _, err := fn(ctx, nil); err == nil {
		t.Fatal("expected error on empty userIDs slice")
	}

	// 2. Only invalid IDs (<10 chars, "0", empty)
	if _, err := fn(ctx, []string{"", "0", "123", "99999999"}); err == nil {
		t.Fatal("expected error when no valid IDs are provided")
	}
}

func TestGetMiniProfilesNormalizationAndBatching(t *testing.T) {
	env := testenv.Load(t)
	uidA := env.UIDA
	gidA := env.GIDA
	if gidA == "" {
		t.Skip("TEST_ZALO_GIDA not set in environment")
	}

	secretBytes := []byte("1234567890123456") // 16 bytes AES key
	secretBase64 := session.SecretKey("MTIzNDU2Nzg5MDEyMzQ1Ng==")

	var reqCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&reqCount, 1)

		if r.Method != http.MethodPost {
			http.Error(w, "expected POST", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		encParams := r.FormValue("params")
		if encParams == "" {
			http.Error(w, "missing params", http.StatusBadRequest)
			return
		}

		plain, err := cryptox.DecodeAESCBC(secretBytes, encParams)
		if err != nil {
			http.Error(w, "decrypt failed: "+err.Error(), http.StatusBadRequest)
			return
		}

		var payload struct {
			FriendIDs  []string `json:"friend_ids"`
			AvatarSize int      `json:"avatar_size"`
			IncInvalid int      `json:"incInvalid"`
			SrcReq     int      `json:"srcReq"`
		}
		if err := json.Unmarshal(plain, &payload); err != nil {
			http.Error(w, "unmarshal failed: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Verify chunk size does not exceed 100
		if len(payload.FriendIDs) > 100 {
			http.Error(w, fmt.Sprintf("chunk size %d exceeds 100", len(payload.FriendIDs)), http.StatusBadRequest)
			return
		}

		// Build response map for requested IDs
		profilesMap := make(map[string]any, len(payload.FriendIDs))
		for _, id := range payload.FriendIDs {
			if id == uidA {
				profilesMap[id] = map[string]any{
					"userId":      "", // Intentionally empty to test auto-population
					"displayName": "", // Intentionally empty to test fallback to zaloName
					"zaloName":    "ZaloUser",
					"avatar":      "https://example.com/avatar.jpg",
					"globalId":    gidA,
					"bizPkg": map[string]any{
						"label": "business",
					},
				}
			} else {
				profilesMap[id] = map[string]any{
					"userId":      id,
					"displayName": "User " + id,
					"zaloName":    "User " + id,
					"globalId":    "GID_" + id,
				}
			}
		}

		innerResp := map[string]any{
			"error_code":    0,
			"error_message": "Success",
			"data":          profilesMap,
		}
		innerBytes, _ := json.Marshal(innerResp)

		encData, err := cryptox.EncodeAESCBC(secretBytes, string(innerBytes), cryptox.EncryptTypeBase64)
		if err != nil {
			http.Error(w, "encode failed", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error_code": 0,
			"data":       encData,
		})
	}))
	defer server.Close()

	sc := session.NewContext()
	sc.SealLogin(session.Seal{
		UID:       uidA,
		UserAgent: "test-user-agent",
		SecretKey: secretBase64,
		Settings:  &session.Settings{},
		LoginInfo: &session.LoginInfo{
			ZpwServiceMapV3: session.ZpwServiceMap{
				Profile: []string{server.URL},
			},
		},
	})

	fn, err := getMiniProfilesFactory(sc, &api{sc: sc})
	if err != nil {
		t.Fatalf("factory failed: %v", err)
	}

	ctx := context.Background()

	// 1. Single profile test (UID_A)
	resSingle, err := fn(ctx, []string{uidA})
	if err != nil {
		t.Fatalf("GetMiniProfiles(uidA) error: %v", err)
	}

	prof, exists := resSingle[uidA]
	if !exists {
		t.Fatalf("expected profile for %s in response", uidA)
	}
	if prof.UserID != uidA {
		t.Errorf("expected UserID %q, got %q", uidA, prof.UserID)
	}
	if prof.DisplayName != "ZaloUser" {
		t.Errorf("expected DisplayName fallback to 'ZaloUser', got %q", prof.DisplayName)
	}
	if prof.GlobalID != gidA {
		t.Errorf("expected GlobalID %q, got %q", gidA, prof.GlobalID)
	}
	if prof.BizInfo == nil || prof.BizInfo["label"] != "business" {
		t.Errorf("expected bizPkg converted to BizInfo, got: %v", prof.BizInfo)
	}
	if prof.BizPkg != nil {
		t.Errorf("expected BizPkg to be nil after normalization, got: %v", prof.BizPkg)
	}

	// 2. Batch test (> 100 IDs: 105 total IDs should trigger exactly 2 HTTP requests)
	atomic.StoreInt32(&reqCount, 0)
	var multiIDs []string
	multiIDs = append(multiIDs, uidA)
	for i := 1; i <= 104; i++ {
		// generate 10-digit IDs for batching test
		multiIDs = append(multiIDs, fmt.Sprintf("10000000%02d", i))
	}

	resMulti, err := fn(ctx, multiIDs)
	if err != nil {
		t.Fatalf("GetMiniProfiles(105 IDs) error: %v", err)
	}

	if count := atomic.LoadInt32(&reqCount); count != 2 {
		t.Errorf("expected exactly 2 requests for 105 IDs (chunks of 100), got %d", count)
	}

	if len(resMulti) != 105 {
		t.Errorf("expected 105 merged profiles, got %d", len(resMulti))
	}
}
