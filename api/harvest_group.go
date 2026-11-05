package api

import (
	"context"

	"github.com/luannt2002/zalo-api-unofficial/errs"
)

// GroupHarvestResult chứa kết quả bóc tách thông tin nhóm và toàn bộ GID thành viên.
// Thực hiện trọn gói chỉ trong đúng 2 network request (1 getmg-v2 + 1 getminiprofiles batch 100).
type GroupHarvestResult struct {
	GroupID       string                  `json:"groupId"`
	GroupGlobalID string                  `json:"groupGlobalId"`
	TotalMember   int                     `json:"totalMember"`
	MemberUIDs    []string                `json:"memberUids"`
	Profiles      GetMiniProfilesResponse `json:"profiles"`
	Pairs         map[string]string       `json:"pairs"` // UID/GroupID -> GID
}

// HarvestGroup thực hiện thu hoạch nhóm trọn gói:
// 1. Gọi getmg-v2 để lấy thông tin nhóm, Group GID và memVerList (danh sách member UID).
// 2. Gọi getminiprofiles để lấy GID thật 100% của từng member (tránh bẫy echo-viewer của /group/members).
func (a *api) HarvestGroup(ctx context.Context, groupID string) (*GroupHarvestResult, error) {
	if groupID == "" {
		return nil, errs.NewZCA("missing group ID", "api.HarvestGroup")
	}

	// 1. Lấy thông tin nhóm (getmg-v2)
	groupInfoResp, err := a.GetGroupInfo(ctx, groupID)
	if err != nil {
		return nil, err
	}

	info, exists := groupInfoResp.GridInfoMap[groupID]
	if !exists {
		return nil, errs.NewZCA("group not found in response", "api.HarvestGroup")
	}

	res := &GroupHarvestResult{
		GroupID:       groupID,
		GroupGlobalID: info.GlobalID,
		TotalMember:   info.TotalMember,
		MemberUIDs:    info.MemVerList,
		Pairs:         make(map[string]string),
	}

	if info.GlobalID != "" {
		res.Pairs[groupID] = info.GlobalID
	}

	// 2. Thu hoạch GID thật và MiniProfile cho toàn bộ member
	if len(info.MemVerList) > 0 {
		profiles, err := a.GetMiniProfiles(ctx, info.MemVerList)
		if err != nil {
			return nil, err
		}
		res.Profiles = profiles
		for uid, prof := range profiles {
			if prof.GlobalID != "" {
				res.Pairs[uid] = prof.GlobalID
			}
		}
	}

	return res, nil
}

