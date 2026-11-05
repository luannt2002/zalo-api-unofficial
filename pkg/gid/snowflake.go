package gid

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"
)

const (
	// GIDAlphabet là bảng chữ cái Base32Hex dùng để biểu diễn GID 32 ký tự của Zalo.
	GIDAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUV"

	// XORToolKey là khóa XOR đã giải mã dùng trong công thức che-dữ-liệu nội bộ (col3 = uid ^ K).
	XORToolKey uint64 = 0x20344cae784c20d6
)

var (
	// SnowflakeEpoch là mốc thời gian epoch của thuật toán sinh ID Snowflake của Zalo:
	// 2011-01-10T17:41:26.666Z (đã được chứng minh độc lập qua 344/344 mẫu).
	SnowflakeEpoch = time.Date(2011, time.January, 10, 17, 41, 26, 666000000, time.UTC)
)

// UIDToTime trích xuất thời điểm tạo viewer-identity từ 19 chữ số UID (snowflake: msTS39 << 24 | rand24).
func UIDToTime(uid string) (time.Time, error) {
	u, err := strconv.ParseUint(uid, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("gid: parse uid %q: %w", uid, err)
	}
	ms := int64(u >> 24)
	return SnowflakeEpoch.Add(time.Duration(ms) * time.Millisecond), nil
}

// UIDBits kiểm tra các bit đặc trưng của snowflake Zalo:
// - bit63 phải bằng 0 (dương)
// - bit15 ngẫu nhiên phải bằng 0
func UIDBits(uid string) (bit63 bool, randBit15 bool, err error) {
	u, err := strconv.ParseUint(uid, 10, 64)
	if err != nil {
		return false, false, fmt.Errorf("gid: parse uid %q: %w", uid, err)
	}
	bit63 = (u >> 63) == 0
	randBit15 = ((u >> 15) & 1) == 0
	return bit63, randBit15, nil
}

// IsWellFormedGID kiểm tra xem một chuỗi GID có đúng cấu trúc chuẩn của Zalo hay không:
// - Độ dài đúng 32 ký tự
// - Mọi ký tự thuộc GIDAlphabet (Base32Hex)
// - Byte đệm cuối (pad-byte) = 0 (tức blob & 0xFF == 0)
func IsWellFormedGID(gid string) bool {
	if len(gid) != 32 {
		return false
	}

	blob := new(big.Int)
	five := big.NewInt(5)

	for i := 0; i < len(gid); i++ {
		c := rune(gid[i])
		idx := strings.IndexRune(GIDAlphabet, c)
		if idx < 0 {
			return false
		}
		blob.Lsh(blob, uint(five.Uint64()))
		blob.Or(blob, big.NewInt(int64(idx)))
	}

	padByte := new(big.Int).And(blob, big.NewInt(0xFF))
	return padByte.Uint64() == 0
}

// UIDToToolCol3 mã hóa UID thành cột toolCol3 bằng khóa XOR nội bộ.
func UIDToToolCol3(uid string) (string, error) {
	u, err := strconv.ParseUint(uid, 10, 64)
	if err != nil {
		return "", fmt.Errorf("gid: parse uid %q: %w", uid, err)
	}
	res := u ^ XORToolKey
	return strconv.FormatUint(res, 10), nil
}

// GIDToBlob chuyển đổi chuỗi GID 32 ký tự (Base32Hex) thành số nguyên lớn 160-bit (*big.Int).
func GIDToBlob(gid string) (*big.Int, error) {
	if len(gid) != 32 {
		return nil, fmt.Errorf("gid: invalid GID length %d (expected 32)", len(gid))
	}

	blob := new(big.Int)
	for i := 0; i < len(gid); i++ {
		idx := strings.IndexByte(GIDAlphabet, gid[i])
		if idx < 0 {
			return nil, fmt.Errorf("gid: invalid character %q in GID", gid[i])
		}
		blob.Lsh(blob, 5)
		blob.Or(blob, big.NewInt(int64(idx)))
	}
	return blob, nil
}

// BlobToGID chuyển đổi số nguyên lớn 160-bit thành chuỗi GID 32 ký tự (Base32Hex).
func BlobToGID(blob *big.Int) string {
	var sb strings.Builder
	sb.Grow(32)

	mask5 := big.NewInt(31) // 0x1F
	cur := new(big.Int).Set(blob)

	chars := make([]byte, 32)
	for i := 31; i >= 0; i-- {
		val := new(big.Int).And(cur, mask5).Int64()
		chars[i] = GIDAlphabet[val]
		cur.Rsh(cur, 5)
	}

	return string(chars)
}

// GIDPayloadBytes trích xuất 19 bytes payload của GID từ blob 160-bit (bỏ qua byte đệm cuối).
func GIDPayloadBytes(blob *big.Int) []byte {
	payload := make([]byte, 19)
	for b := 0; b < 19; b++ {
		shift := uint(8 * (18 - b) + 8) // bỏ qua 8-bit pad cuối
		tmp := new(big.Int).Rsh(blob, shift)
		payload[b] = byte(new(big.Int).And(tmp, big.NewInt(0xFF)).Uint64())
	}
	return payload
}

// GIDCohortWindow trích xuất trường cohort W 32-bit (unsigned) từ byte {4, 7, 8, 10} của payload.
// Trong thực nghiệm REA, W là đặc trưng nhóm (cohort field), tái sử dụng xuyên user <-> group.
func GIDCohortWindow(blob *big.Int) uint32 {
	payload := GIDPayloadBytes(blob)
	windowBytes := []int{4, 7, 8, 10}
	var w uint32
	for _, idx := range windowBytes {
		w = (w << 8) | uint32(payload[idx])
	}
	return w
}

// ToolCol3ToUID giải mã cột toolCol3 về UID gốc bằng khóa XOR nội bộ.
func ToolCol3ToUID(col3 string) (string, error) {
	c, err := strconv.ParseUint(col3, 10, 64)
	if err != nil {
		return "", fmt.Errorf("gid: parse col3 %q: %w", col3, err)
	}
	res := c ^ XORToolKey
	return strconv.FormatUint(res, 10), nil
}

