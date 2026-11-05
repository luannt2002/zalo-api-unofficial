package media

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// HostClass phân loại miền CDN và máy chủ của Zalo.
type HostClass string

const (
	HostClassZaloCDN   HostClass = "zalo-cdn"
	HostClassZalo      HostClass = "zalo"
	HostClassZaloVideo HostClass = "zalo-video"
	HostClassOther     HostClass = "other"
)

// ClassificationType xác định mức bảo đảm quyền truy cập của URL media.
type ClassificationType string

const (
	ClassCapabilityCandidate ClassificationType = "CAPABILITY_CANDIDATE"
	ClassUnverifiedURL       ClassificationType = "UNVERIFIED_URL"
)

var (
	alnumRunRegex = regexp.MustCompile(`[A-Za-z0-9]{20,}`)
	epochRegex    = regexp.MustCompile(`(?:^|[^0-9])([0-9]{10}|[0-9]{13})(?:[^0-9]|$)`)
	hex32Regex    = regexp.MustCompile(`^[0-9a-fA-F]{32}$`)
	digitsRegex   = regexp.MustCompile(`^[0-9]+$`)
)

// KeyCandidate biểu diễn khóa chữ ký hoặc token nhúng trong đường dẫn/tham số URL.
type KeyCandidate struct {
	Length int    `json:"len"`
	Hash16 string `json:"h16"`
}

// URLClassification chứa kết quả phân tích cấu trúc, chữ ký và thời hạn của link media Zalo.
type URLClassification struct {
	OK                  bool               `json:"ok"`
	Reason              string             `json:"reason,omitempty"`
	Host                string             `json:"host"`
	HostClass           HostClass          `json:"host_class"`
	PathSegments        int                `json:"path_segments"`
	HasQuery            bool               `json:"has_query"`
	Class               ClassificationType `json:"class"`
	SignatureHint       bool               `json:"signature_hint"`
	TimestampCandidates []time.Time        `json:"timestamp_candidates_utc"`
	KeyCandidates       []KeyCandidate     `json:"key_candidates"`
	ExpiresAt           *time.Time         `json:"expires_at,omitempty"`
	IsExpired           bool               `json:"is_expired"`
	TimeRemaining       time.Duration      `json:"time_remaining"`
	Verdict             string             `json:"verdict"`
}

// DetermineHostClass ánh xạ hostname sang phân lớp hạ tầng Zalo.
func DetermineHostClass(host string) HostClass {
	lower := strings.ToLower(strings.TrimRight(host, "."))
	if lower == "zadn.vn" || strings.HasSuffix(lower, ".zadn.vn") ||
		lower == "zdn.vn" || strings.HasSuffix(lower, ".zdn.vn") {
		return HostClassZaloCDN
	}
	if lower == "zalo.me" || strings.HasSuffix(lower, ".zalo.me") ||
		lower == "zaloapp.com" || strings.HasSuffix(lower, ".zaloapp.com") {
		return HostClassZalo
	}
	if lower == "zalo.video" || strings.HasSuffix(lower, ".zalo.video") {
		return HostClassZaloVideo
	}
	return HostClassOther
}

// Classify phân tích shape và trích xuất chữ ký/thời hạn của link media theo bộ nhận dạng REA.
func Classify(rawURL string) *URLClassification {
	rawURL = strings.TrimSpace(rawURL)
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return &URLClassification{
			OK:     false,
			Reason: "not-url",
		}
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return &URLClassification{
			OK:     false,
			Reason: "unsupported-scheme",
		}
	}

	host := strings.ToLower(strings.TrimRight(u.Hostname(), "."))
	hClass := DetermineHostClass(host)

	var segments []string
	for _, s := range strings.Split(u.Path, "/") {
		if s != "" {
			segments = append(segments, s)
		}
	}

	full := u.Path + "?" + u.RawQuery

	// 1. Tìm key candidates: chuỗi alnum >= 20 ký tự (loại trừ md5 32 hex và số thuần)
	var keys []KeyCandidate
	for _, match := range alnumRunRegex.FindAllString(full, -1) {
		if hex32Regex.MatchString(match) || digitsRegex.MatchString(match) {
			continue
		}
		h := sha256.Sum256([]byte(match))
		h16 := hex.EncodeToString(h[:])[:16]
		keys = append(keys, KeyCandidate{
			Length: len(match),
			Hash16: h16,
		})
	}

	// 2. Tìm timestamp candidate: 10 số (giây) hoặc 13 số (mili-giây) trong khoảng 2015 -> 2040
	var timestamps []time.Time
	matches := epochRegex.FindAllStringSubmatch(full, -1)
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		digits := m[1]
		val, err := strconv.ParseInt(digits, 10, 64)
		if err != nil {
			continue
		}
		var sec int64
		if len(digits) == 13 {
			sec = val / 1000
		} else {
			sec = val
		}

		if sec >= 1420070400 && sec <= 2208988800 {
			t := time.Unix(sec, 0).UTC()
			timestamps = append(timestamps, t)
		}
	}

	// 3. Gợi ý chữ ký qua query params
	queryParams := u.Query()
	sigHint := false
	for _, k := range []string{"key", "token", "sig", "signature"} {
		if _, ok := queryParams[k]; ok {
			sigHint = true
			break
		}
	}

	classification := ClassUnverifiedURL
	if hClass != HostClassOther && (sigHint || len(keys) > 0) {
		classification = ClassCapabilityCandidate
	}

	out := &URLClassification{
		OK:                  true,
		Host:                host,
		HostClass:           hClass,
		PathSegments:        len(segments),
		HasQuery:            u.RawQuery != "",
		Class:               classification,
		SignatureHint:       sigHint,
		TimestampCandidates: timestamps,
		KeyCandidates:       keys,
		Verdict:             "Nhận dạng cấu trúc URL theo REA; cần verify qua network khi tải thực tế.",
	}

	if len(timestamps) > 0 {
		exp := timestamps[0]
		out.ExpiresAt = &exp
		now := time.Now().UTC()
		out.IsExpired = now.After(exp)
		if !out.IsExpired {
			out.TimeRemaining = exp.Sub(now)
		}
	}

	return out
}

// Shape tạo chuỗi mô tả cấu trúc ẩn danh an toàn không làm lộ token hay chữ ký.
func Shape(rawURL string) string {
	res := Classify(rawURL)
	if !res.OK {
		return "INVALID_URL"
	}
	u, _ := url.Parse(rawURL)
	var segments []string
	for _, s := range strings.Split(u.Path, "/") {
		if s != "" {
			segments = append(segments, s)
		}
	}

	queryPart := ""
	if u.RawQuery != "" {
		queryPart = "?<query>"
	}
	return fmt.Sprintf("%s://%s/<%d-segments>%s", u.Scheme, u.Hostname(), len(segments), queryPart)
}

