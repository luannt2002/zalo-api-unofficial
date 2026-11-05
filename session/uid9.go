package session

import (
	"net/http"
	"net/url"
	"strings"
)

// ExtractUID9 extracts the permanent legacy account ID (typically 9 digits)
// from a zpw_sek or zpsid cookie value (tag.UID9.selector.secret).
// Returns empty string if value does not match the observed protocol shape.
func ExtractUID9(cookieValue string) string {
	parts := strings.Split(cookieValue, ".")
	if len(parts) >= 3 && len(parts[1]) >= 6 && len(parts[1]) <= 12 {
		for _, c := range parts[1] {
			if c < '0' || c > '9' {
				return ""
			}
		}
		return parts[1]
	}
	return ""
}

// ExtractUID9FromJar inspects cookies in jar for chat.zalo.me, zalo.me, and id.zalo.me
// looking for zpw_sek or zpsid to extract the permanent UID9.
func ExtractUID9FromJar(jar http.CookieJar) string {
	if jar == nil {
		return ""
	}
	checkURLs := []string{"https://chat.zalo.me", "https://zalo.me", "https://id.zalo.me"}
	for _, uStr := range checkURLs {
		u, err := url.Parse(uStr)
		if err != nil {
			continue
		}
		for _, cookie := range jar.Cookies(u) {
			if cookie.Name == "zpw_sek" || cookie.Name == "zpsid" {
				if id := ExtractUID9(cookie.Value); id != "" {
					return id
				}
			}
		}
	}
	return ""
}

