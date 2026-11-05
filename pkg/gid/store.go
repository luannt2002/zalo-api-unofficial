package gid

import (
	"encoding/json"
	"sync"
)

// Store quản lý bộ nhớ đệm hai chiều (bidirectional cache) giữa local UID/GroupID và global GID.
// Hỗ trợ lưu trữ riêng biệt thực thể User và Group khớp thiết kế table_global_id của Zalo (type 0: user, type 1: group).
// Đảm bảo an toàn đồng thời (thread-safe).
type Store struct {
	mu          sync.RWMutex
	uid2gid     map[string]string
	gid2uid     map[string]string
	group2gid   map[string]string
	gid2group   map[string]string
	pairMasks   map[string]uint64
}

// NewStore tạo một Store mới rỗng.
func NewStore() *Store {
	return &Store{
		uid2gid:   make(map[string]string),
		gid2uid:   make(map[string]string),
		group2gid: make(map[string]string),
		gid2group: make(map[string]string),
		pairMasks: make(map[string]uint64),
	}
}

// GetOrComputeMask trả về mask XOR giữa 2 send2me_id (A và B), tự động tính toán và cache.
func (s *Store) GetOrComputeMask(send2meA, send2meB string) (uint64, error) {
	key := send2meA + ":" + send2meB
	if send2meA > send2meB {
		key = send2meB + ":" + send2meA
	}

	s.mu.RLock()
	mask, ok := s.pairMasks[key]
	s.mu.RUnlock()
	if ok {
		return mask, nil
	}

	computed, err := MaskFromSend2me(send2meA, send2meB)
	if err != nil {
		return 0, err
	}

	s.mu.Lock()
	s.pairMasks[key] = computed
	s.mu.Unlock()
	return computed, nil
}

// Set lưu một cặp quan hệ User UID <-> GID vào store.
func (s *Store) Set(uid, gid string) {
	s.SetUser(uid, gid)
}

// SetUser lưu một cặp quan hệ User UID <-> GID vào store.
func (s *Store) SetUser(uid, gid string) {
	if uid == "" || gid == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.uid2gid[uid] = gid
	s.gid2uid[gid] = uid
}

// GetGID trả về GID tương ứng với User UID nếu có trong cache.
func (s *Store) GetGID(uid string) (string, bool) {
	return s.GetUserGID(uid)
}

// GetUserGID trả về GID tương ứng với User UID nếu có trong cache.
func (s *Store) GetUserGID(uid string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	gid, ok := s.uid2gid[uid]
	return gid, ok
}

// GetUID trả về User UID tương ứng với GID nếu có trong cache.
func (s *Store) GetUID(gid string) (string, bool) {
	return s.GetUserUID(gid)
}

// GetUserUID trả về User UID tương ứng với GID nếu có trong cache.
func (s *Store) GetUserUID(gid string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	uid, ok := s.gid2uid[gid]
	return uid, ok
}

// SetGroup lưu một cặp quan hệ GroupID <-> Group GID vào store.
func (s *Store) SetGroup(groupID, gid string) {
	if groupID == "" || gid == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.group2gid[groupID] = gid
	s.gid2group[gid] = groupID
}

// GetGroupGID trả về GID tương ứng với GroupID nếu có trong cache.
func (s *Store) GetGroupGID(groupID string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	gid, ok := s.group2gid[groupID]
	return gid, ok
}

// GetGroupID trả về GroupID tương ứng với Group GID nếu có trong cache.
func (s *Store) GetGroupID(gid string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	groupID, ok := s.gid2group[gid]
	return groupID, ok
}

// UserCount trả về số lượng cặp User UID/GID trong store.
func (s *Store) UserCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.uid2gid)
}

// GroupCount trả về số lượng cặp Group ID/GID trong store.
func (s *Store) GroupCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.group2gid)
}

// Len trả về tổng số lượng cặp thực thể (cả users và groups) trong store.
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.uid2gid) + len(s.group2gid)
}

type storeSnapshot struct {
	Users       map[string]string `json:"users"`
	UsersByGID  map[string]string `json:"usersByGid"`
	Groups      map[string]string `json:"groups"`
	GroupsByGID map[string]string `json:"groupsByGid,omitempty"`
}

// MarshalJSON xuất toàn bộ cache ra định dạng JSON tương thích my-id-store.json.
func (s *Store) MarshalJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snap := storeSnapshot{
		Users:       make(map[string]string, len(s.uid2gid)),
		UsersByGID:  make(map[string]string, len(s.gid2uid)),
		Groups:      make(map[string]string, len(s.group2gid)),
		GroupsByGID: make(map[string]string, len(s.gid2group)),
	}
	for k, v := range s.uid2gid {
		snap.Users[k] = v
	}
	for k, v := range s.gid2uid {
		snap.UsersByGID[k] = v
	}
	for k, v := range s.group2gid {
		snap.Groups[k] = v
	}
	for k, v := range s.gid2group {
		snap.GroupsByGID[k] = v
	}

	return json.Marshal(snap)
}

// UnmarshalJSON nạp dữ liệu từ JSON vào Store.
func (s *Store) UnmarshalJSON(data []byte) error {
	var snap storeSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.uid2gid = make(map[string]string, len(snap.Users))
	s.gid2uid = make(map[string]string, len(snap.Users))
	s.group2gid = make(map[string]string, len(snap.Groups))
	s.gid2group = make(map[string]string, len(snap.Groups))

	for uid, gid := range snap.Users {
		s.uid2gid[uid] = gid
		s.gid2uid[gid] = uid
	}
	for gid, uid := range snap.UsersByGID {
		s.gid2uid[gid] = uid
		if _, exists := s.uid2gid[uid]; !exists {
			s.uid2gid[uid] = gid
		}
	}
	for groupID, gid := range snap.Groups {
		s.group2gid[groupID] = gid
		s.gid2group[gid] = groupID
	}
	for gid, groupID := range snap.GroupsByGID {
		s.gid2group[gid] = groupID
		if _, exists := s.group2gid[groupID]; !exists {
			s.group2gid[groupID] = gid
		}
	}

	return nil
}
