package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/luannt2002/zalo-api-unofficial/errs"
	"github.com/luannt2002/zalo-api-unofficial/internal/httpx"
	"github.com/luannt2002/zalo-api-unofficial/internal/jsonx"
	"github.com/luannt2002/zalo-api-unofficial/session"
)

const maxMiniProfileBatchSize = 100

type MiniProfileType int

const (
	MiniProfileTypeUser MiniProfileType = 0 // Người dùng cá nhân bình thường
	MiniProfileTypeOA   MiniProfileType = 5 // Official Account
	MiniProfileTypeZOA  MiniProfileType = 7 // Official Account thuộc Zalo (vd: Zalo To-Do, zCloud)
)

type (
	// MiniProfile chứa thông tin profile cơ bản và GlobalID (GID) thật của một user.
	// Khác với /api/social/group/members (bị echo GID của viewer), getminiprofiles trả về GID thật của từng member.
	MiniProfile struct {
		UserID      string          `json:"userId,omitempty"`
		DisplayName string          `json:"displayName,omitempty"`
		ZaloName    string          `json:"zaloName,omitempty"`
		Avatar      string          `json:"avatar,omitempty"`
		GlobalID    string          `json:"globalId,omitempty"`
		Type        MiniProfileType `json:"type,omitempty"`
		BizInfo     map[string]any  `json:"bizInfo,omitempty"`
		BizPkg      map[string]any  `json:"bizPkg,omitempty"`
		IsFriend    *bool           `json:"isFr,omitempty"`
	}

	// GetMiniProfilesResponse là map ánh xạ từ UID -> MiniProfile.
	GetMiniProfilesResponse map[string]MiniProfile

	// MiniProfileOptions chứa cấu hình tùy chọn khi gọi API getminiprofiles.
	MiniProfileOptions struct {
		AvatarSize int // Kích thước avatar (mặc định: 120 như client web)
		SrcRequest int // Mã nguồn request (mặc định: -1)
	}

	// GetMiniProfilesFn đại diện cho hàm gọi API getminiprofiles.
	GetMiniProfilesFn = func(ctx context.Context, userIDs []string, opts ...MiniProfileOptions) (GetMiniProfilesResponse, error)
)

func (a *api) GetMiniProfiles(ctx context.Context, userIDs []string, opts ...MiniProfileOptions) (GetMiniProfilesResponse, error) {
	return a.e.GetMiniProfiles(ctx, userIDs, opts...)
}

var getMiniProfilesFactory = apiFactory[GetMiniProfilesResponse, GetMiniProfilesFn]()(
	func(a *api, sc session.Context, u factoryUtils[GetMiniProfilesResponse]) (GetMiniProfilesFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("profile"), "")
		serviceURL := u.MakeURL(base+"/api/social/friend/getminiprofiles", nil, true)

		return func(ctx context.Context, userIDs []string, opts ...MiniProfileOptions) (GetMiniProfilesResponse, error) {
			if len(userIDs) == 0 {
				return nil, errs.NewZCA("missing user IDs", "api.GetMiniProfiles")
			}

			avatarSize := 120
			srcReq := -1
			if len(opts) > 0 {
				if opts[0].AvatarSize > 0 {
					avatarSize = opts[0].AvatarSize
				}
				if opts[0].SrcRequest != 0 {
					srcReq = opts[0].SrcRequest
				}
			}

			// Lọc và loại trừ trùng lặp: chỉ nhận UID chuỗi >= 10 ký tự và khác "0"
			seen := make(map[string]struct{}, len(userIDs))
			var validIDs []string
			for _, id := range userIDs {
				id = strings.TrimSpace(id)
				if len(id) >= 10 && id != "0" {
					if _, exists := seen[id]; !exists {
						seen[id] = struct{}{}
						validIDs = append(validIDs, id)
					}
				}
			}

			if len(validIDs) == 0 {
				return nil, errs.NewZCA("no valid user IDs provided", "api.GetMiniProfiles")
			}

			merged := make(GetMiniProfilesResponse, len(validIDs))

			// Tự động phân chia batch tối đa 100 UIDs/request (đặc tả client web Zalo: class N, chunk1 byte 8076507+)
			for i := 0; i < len(validIDs); i += maxMiniProfileBatchSize {
				end := i + maxMiniProfileBatchSize
				if end > len(validIDs) {
					end = len(validIDs)
				}
				chunk := validIDs[i:end]

				payload := map[string]any{
					"friend_ids":  chunk,
					"avatar_size": avatarSize,
					"incInvalid":  1,
					"srcReq":      srcReq,
				}

				enc, err := u.EncodeAES(jsonx.Stringify(payload))
				if err != nil {
					return nil, errs.WrapZCA("failed to encrypt params", "api.GetMiniProfiles", err)
				}

				body := httpx.BuildFormBody(map[string]string{"params": enc})
				resp, err := u.Request(ctx, serviceURL, &httpx.RequestOptions{
					Method: http.MethodPost,
					Body:   body,
				})
				if err != nil {
					return nil, err
				}

				batchRes, err := u.Resolve(resp, true)
				if err != nil {
					resp.Body.Close()
					return nil, err
				}
				resp.Body.Close()

				// Chuẩn hóa profile:
				// 1. Gán UserID nếu server không gửi kèm trong field
				// 2. Chuyển đổi bizPkg -> bizInfo
				// 3. Fallback: displayName -> zaloName nếu displayName rỗng
				for uid, profile := range batchRes {
					if profile.UserID == "" {
						profile.UserID = uid
					}
					if profile.BizPkg != nil && profile.BizInfo == nil {
						profile.BizInfo = profile.BizPkg
						profile.BizPkg = nil
					}
					if profile.DisplayName == "" && profile.ZaloName != "" {
						profile.DisplayName = profile.ZaloName
					}
					merged[uid] = profile
				}
			}

			return merged, nil
		}, nil
	},
)

