package gid

import (
	"fmt"
	"strconv"
)

// MaskFromSend2me tính toán mask XOR giữa hai tài khoản dựa trên send2me_id của họ.
// Khám phá giải mã: send2me_id là ID hội thoại "Cloud của tôi" (uint64),
// và K_AB = send2me_id(A) ^ send2me_id(B).
func MaskFromSend2me(send2meA, send2meB string) (uint64, error) {
	valA, err := strconv.ParseUint(send2meA, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("gid: parse send2meA %q: %w", send2meA, err)
	}
	valB, err := strconv.ParseUint(send2meB, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("gid: parse send2meB %q: %w", send2meB, err)
	}
	return valA ^ valB, nil
}

// TranslateViewerUID chuyển đổi local UID của một entity từ góc nhìn viewer A sang viewer B
// dựa trên send2me_id của hai viewer mà không cần bất kỳ API call nào (0-network).
// Công thức: uid_B = uid_A ^ (send2me_A ^ send2me_B).
func TranslateViewerUID(uidA, send2meA, send2meB string) (string, error) {
	mask, err := MaskFromSend2me(send2meA, send2meB)
	if err != nil {
		return "", err
	}
	uA, err := strconv.ParseUint(uidA, 10, 64)
	if err != nil {
		return "", fmt.Errorf("gid: parse uidA %q: %w", uidA, err)
	}
	uB := uA ^ mask
	return strconv.FormatUint(uB, 10), nil
}

// TranslateViewerUIDUint64 chuyển đổi UID dạng số nguyên uint64 qua mask đã tính trước.
func TranslateViewerUIDUint64(uidA, mask uint64) uint64 {
	return uidA ^ mask
}

// CalibrateMask hiệu chuẩn mask XOR giữa hai viewer từ 1 cặp UID của cùng một người (hoặc cùng một entity).
// Công thức: Mask = UID_A ^ UID_B.
func CalibrateMask(uidA, uidB string) (uint64, error) {
	valA, err := strconv.ParseUint(uidA, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("gid: parse uidA %q: %w", uidA, err)
	}
	valB, err := strconv.ParseUint(uidB, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("gid: parse uidB %q: %w", uidB, err)
	}
	return valA ^ valB, nil
}

// TranslateWithMask chuyển đổi UID dạng chuỗi qua một mask XOR 64-bit đã biết trước.
func TranslateWithMask(uid string, mask uint64) (string, error) {
	val, err := strconv.ParseUint(uid, 10, 64)
	if err != nil {
		return "", fmt.Errorf("gid: parse uid %q: %w", uid, err)
	}
	return strconv.FormatUint(val^mask, 10), nil
}

// TranslateGroupID chuyển đổi ID nhóm (GroupID) giữa hai viewer qua cùng mask XOR (phát hiện Round 11d REA: GroupID cũng per-viewer theo cùng mask).
func TranslateGroupID(groupID string, mask uint64) (string, error) {
	return TranslateWithMask(groupID, mask)
}

