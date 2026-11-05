package session

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/luannt2002/zalo-api-unofficial/pkg/gid"
)

// SessionDiagnostics tóm lược kết quả thanh tra một file session/credentials JSON.
type SessionDiagnostics struct {
	UID9             string     `json:"uid9,omitempty"`
	ViewerUID        string     `json:"viewer_uid,omitempty"`
	ViewerCreatedAt  *time.Time `json:"viewer_created_at,omitempty"`
	HasDeleteMarkers bool       `json:"has_delete_markers"`
	DeleteMarkerKeys []string   `json:"delete_marker_keys,omitempty"`
	CookieCount      int        `json:"cookie_count"`
	Domains          []string   `json:"domains"`
	IMEI             string     `json:"imei,omitempty"`
	UserAgent        string     `json:"user_agent,omitempty"`
}

// InspectSession bóc tách phân tích chi tiết một file session (chứa cookies, imei, userAgent)
// hoàn toàn offline mà không cần thực hiện kết nối mạng tới Zalo theo 5 quy tắc thực chiến REA.
func InspectSession(rawJSON []byte) (*SessionDiagnostics, error) {
	var raw struct {
		Cookies   json.RawMessage `json:"cookies"`
		Cookie    json.RawMessage `json:"cookie"`
		IMEI      string          `json:"imei"`
		UserAgent string          `json:"userAgent"`
		UID       string          `json:"uid"`
	}

	if err := json.Unmarshal(rawJSON, &raw); err != nil {
		return nil, fmt.Errorf("session: invalid json format: %w", err)
	}

	cookieBlob := raw.Cookies
	if len(cookieBlob) == 0 {
		cookieBlob = raw.Cookie
	}

	type genericCookie struct {
		Key     string `json:"key"`
		Name    string `json:"name"`
		Value   string `json:"value"`
		Domain  string `json:"domain"`
		Expires string `json:"expires"`
	}

	var list []genericCookie
	if err := json.Unmarshal(cookieBlob, &list); err != nil {
		var wrapped struct {
			Cookies []genericCookie `json:"cookies"`
		}
		if err2 := json.Unmarshal(cookieBlob, &wrapped); err2 == nil && len(wrapped.Cookies) > 0 {
			list = wrapped.Cookies
		}
	}

	diag := &SessionDiagnostics{
		IMEI:        raw.IMEI,
		UserAgent:   raw.UserAgent,
		ViewerUID:   raw.UID,
		CookieCount: len(list),
	}

	domainMap := make(map[string]bool)
	for _, c := range list {
		name := c.Name
		if name == "" {
			name = c.Key
		}

		if c.Domain != "" {
			domainMap[strings.TrimPrefix(c.Domain, ".")] = true
		}

		if c.Value == "EXPIRED" || strings.Contains(c.Expires, "1970") {
			diag.HasDeleteMarkers = true
			diag.DeleteMarkerKeys = append(diag.DeleteMarkerKeys, fmt.Sprintf("%s@%s", name, c.Domain))
		}

		if diag.UID9 == "" && (name == "zpw_sek" || name == "zpsid") {
			if u9 := ExtractUID9(c.Value); u9 != "" {
				diag.UID9 = u9
			}
		}
	}

	for d := range domainMap {
		diag.Domains = append(diag.Domains, d)
	}
	sort.Strings(diag.Domains)

	if diag.ViewerUID != "" {
		if ts, err := gid.UIDToTime(diag.ViewerUID); err == nil {
			diag.ViewerCreatedAt = &ts
		}
	}

	return diag, nil
}

// CookieChangesResult thống kê chi tiết sự biến thiên của cookie jar (Wave 24).
type CookieChangesResult struct {
	Added           int `json:"added"`
	Removed         int `json:"removed"`
	ValueChanged     int `json:"valueChanged"`
	MetadataChanged int `json:"metadataChanged"`
}

// CookieRecord biểu diễn một bản ghi cookie độc lập dùng cho việc so sánh phân loại.
type CookieRecord struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	Expires  string `json:"expires,omitempty"`
	Secure   bool   `json:"secure,omitempty"`
	HttpOnly bool   `json:"httpOnly,omitempty"`
}

// ClassifyCookieChanges đối chiếu 2 tập cookies (trước và sau) để phân loại chính xác:
// Added, Removed, ValueChanged, MetadataChanged theo đặc tả session-contract (Wave 24).
func ClassifyCookieChanges(before, after []CookieRecord) CookieChangesResult {
	identity := func(c CookieRecord) string {
		path := c.Path
		if path == "" {
			path = "/"
		}
		domain := strings.ToLower(c.Domain)
		domain = strings.TrimPrefix(domain, ".")
		return fmt.Sprintf("%s|%s|%s", c.Key, domain, path)
	}

	oldMap := make(map[string]CookieRecord, len(before))
	for _, c := range before {
		oldMap[identity(c)] = c
	}

	freshMap := make(map[string]CookieRecord, len(after))
	for _, c := range after {
		freshMap[identity(c)] = c
	}

	allKeys := make(map[string]bool)
	for k := range oldMap {
		allKeys[k] = true
	}
	for k := range freshMap {
		allKeys[k] = true
	}

	res := CookieChangesResult{}
	for k := range allKeys {
		a, hasA := oldMap[k]
		b, hasB := freshMap[k]

		if !hasA {
			res.Added++
		} else if !hasB {
			res.Removed++
		} else if a.Value != b.Value {
			res.ValueChanged++
		} else if a.Expires != b.Expires || a.Secure != b.Secure || a.HttpOnly != b.HttpOnly {
			res.MetadataChanged++
		}
	}

	return res
}


