package api

import (
	"context"
	"testing"

	"github.com/luannt2002/zalo-api-unofficial/internal/testenv"
	"github.com/luannt2002/zalo-api-unofficial/model"
	"github.com/luannt2002/zalo-api-unofficial/session"
)

func TestHarvestGroup(t *testing.T) {
	env := testenv.Load(t)
	uidA := env.UIDA
	gidA := env.GIDA
	if gidA == "" {
		t.Skip("TEST_ZALO_GIDA not set in environment")
	}

	groupID := "8910587406831154166"
	groupGID := "BPAH0123456789ABCDEF012345678900"

	sc := session.NewContext()
	sc.SealLogin(session.Seal{
		UID:       uidA,
		UserAgent: "test-user-agent",
		SecretKey: session.SecretKey("dGVzdC1zZWNyZXQta2V5LTE2Yg=="),
		Settings:  &session.Settings{},
	})

	a := &api{
		sc: sc,
		e: endpoints{
			GetGroupInfo: func(ctx context.Context, groupIDs ...string) (*GetGroupInfoResponse, error) {
				return &GetGroupInfoResponse{
					GridInfoMap: map[string]GroupInfo{
						groupID: {
							GroupInfo: model.GroupInfo{
								GlobalID:    groupGID,
								TotalMember: 1,
							},
							MemVerList: []string{uidA},
						},
					},
				}, nil
			},
			GetMiniProfiles: func(ctx context.Context, userIDs []string, opts ...MiniProfileOptions) (GetMiniProfilesResponse, error) {
				return GetMiniProfilesResponse{
					uidA: {
						UserID:      uidA,
						DisplayName: "User A",
						GlobalID:    gidA,
					},
				}, nil
			},
		},
	}

	ctx := context.Background()
	result, err := a.HarvestGroup(ctx, groupID)
	if err != nil {
		t.Fatalf("HarvestGroup failed: %v", err)
	}

	if result.GroupID != groupID {
		t.Errorf("GroupID = %q, expected %q", result.GroupID, groupID)
	}
	if result.GroupGlobalID != groupGID {
		t.Errorf("GroupGlobalID = %q, expected %q", result.GroupGlobalID, groupGID)
	}
	if len(result.MemberUIDs) != 1 || result.MemberUIDs[0] != uidA {
		t.Errorf("MemberUIDs = %v, expected [%s]", result.MemberUIDs, uidA)
	}
	if result.Pairs[groupID] != groupGID {
		t.Errorf("Pairs[%s] = %q, expected %q", groupID, result.Pairs[groupID], groupGID)
	}
	if result.Pairs[uidA] != gidA {
		t.Errorf("Pairs[%s] = %q, expected %q", uidA, result.Pairs[uidA], gidA)
	}
}

func TestResolveUIDViaGID(t *testing.T) {
	env := testenv.Load(t)
	uidA := env.UIDA
	gidA := env.GIDA
	if gidA == "" {
		t.Skip("TEST_ZALO_GIDA not set in environment")
	}

	sc := session.NewContext()
	sc.SealLogin(session.Seal{
		UID:       uidA,
		UserAgent: "test-user-agent",
		SecretKey: session.SecretKey("dGVzdC1zZWNyZXQta2V5LTE2Yg=="),
		Settings:  &session.Settings{},
	})

	a := &api{
		sc: sc,
		e: endpoints{
			DecryptGlobalID: func(ctx context.Context, req DecryptGlobalIDRequest) (DecryptGlobalIDResponse, error) {
				return DecryptGlobalIDResponse{
					gidA: uidA,
				}, nil
			},
		},
	}

	ctx := context.Background()
	res, err := a.ResolveUIDViaGID(ctx, gidA)
	if err != nil {
		t.Fatalf("ResolveUIDViaGID failed: %v", err)
	}

	if res[gidA] != uidA {
		t.Errorf("res[%s] = %q, expected %q", gidA, res[gidA], uidA)
	}

	// Empty IDs
	if _, err := a.ResolveUIDViaGID(ctx); err == nil {
		t.Fatal("expected error on empty global IDs")
	}
}

