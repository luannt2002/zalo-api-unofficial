package gid_test

import (
	"encoding/json"
	"testing"

	"github.com/luannt2002/zalo-api-unofficial/internal/testenv"
	"github.com/luannt2002/zalo-api-unofficial/pkg/gid"
)

func TestStoreUsersAndGroups(t *testing.T) {
	env := testenv.Load(t)
	uidA := env.UIDA
	gidA := env.GIDA
	if gidA == "" {
		t.Skip("TEST_ZALO_GIDA not set in environment")
	}

	s := gid.NewStore()

	// 1. User mapping
	s.SetUser(uidA, gidA)
	if gotGID, ok := s.GetUserGID(uidA); !ok || gotGID != gidA {
		t.Errorf("GetUserGID(%s) = %q, expected %q", uidA, gotGID, gidA)
	}
	if gotUID, ok := s.GetUserUID(gidA); !ok || gotUID != uidA {
		t.Errorf("GetUserUID(%s) = %q, expected %q", gidA, gotUID, uidA)
	}
	if s.UserCount() != 1 {
		t.Errorf("UserCount = %d, expected 1", s.UserCount())
	}

	// 2. Group mapping
	groupID := "8910587406831154166"
	groupGID := "BPAH0123456789ABCDEF012345678900"
	s.SetGroup(groupID, groupGID)
	if gotGID, ok := s.GetGroupGID(groupID); !ok || gotGID != groupGID {
		t.Errorf("GetGroupGID(%s) = %q, expected %q", groupID, gotGID, groupGID)
	}
	if gotID, ok := s.GetGroupID(groupGID); !ok || gotID != groupID {
		t.Errorf("GetGroupID(%s) = %q, expected %q", groupGID, gotID, groupID)
	}
	if s.GroupCount() != 1 {
		t.Errorf("GroupCount = %d, expected 1", s.GroupCount())
	}
	if s.Len() != 2 {
		t.Errorf("Len = %d, expected 2", s.Len())
	}

	// 3. JSON round-trip
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("MarshalJSON error: %v", err)
	}

	s2 := gid.NewStore()
	if err := json.Unmarshal(data, s2); err != nil {
		t.Fatalf("UnmarshalJSON error: %v", err)
	}

	if gotGID, ok := s2.GetUserGID(uidA); !ok || gotGID != gidA {
		t.Errorf("s2.GetUserGID(%s) = %q, expected %q", uidA, gotGID, gidA)
	}
	if gotGID, ok := s2.GetGroupGID(groupID); !ok || gotGID != groupGID {
		t.Errorf("s2.GetGroupGID(%s) = %q, expected %q", groupID, gotGID, groupGID)
	}
}

