package zcago

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"

	"github.com/luannt2002/zalo-api-unofficial/errs"
	"github.com/luannt2002/zalo-api-unofficial/session"
)

// Credentials represents authentication data needed for Zalo login.
type Credentials struct {
	IMEI      string       `json:"imei"`
	Cookie    *CookieUnion `json:"cookie"`
	UserAgent string       `json:"userAgent"`
	Language  *string      `json:"language,omitempty"`
}

func NewCredentials(imei string, cookie CookieUnion, userAgent string, language *string) Credentials {
	return Credentials{
		IMEI:      imei,
		Cookie:    &cookie,
		UserAgent: userAgent,
		Language:  language,
	}
}

func (c Credentials) IsValid() bool {
	return len(c.IMEI) > 0 && (c.Cookie == nil || c.Cookie.IsValid()) && len(c.UserAgent) > 0
}

// UID9 extracts the permanent legacy account ID (typically 9 digits) from credentials' cookies.
func (c Credentials) UID9() string {
	if c.Cookie == nil {
		return ""
	}
	for _, ck := range c.Cookie.GetCookies() {
		if ck.Name == "zpw_sek" || ck.Name == "zpsid" {
			if id := session.ExtractUID9(ck.Value); id != "" {
				return id
			}
		}
	}
	return ""
}

// ExtractUID9 extracts the permanent legacy account ID (typically 9 digits)
// from a zpw_sek or zpsid cookie value (tag.UID9.selector.secret).
func ExtractUID9(cookieValue string) string {
	return session.ExtractUID9(cookieValue)
}

// ExtractUID9FromJar inspects cookies in jar for chat.zalo.me, zalo.me, and id.zalo.me
// looking for zpw_sek or zpsid to extract the permanent UID9.
func ExtractUID9FromJar(jar http.CookieJar) string {
	return session.ExtractUID9FromJar(jar)
}

type SameSite string

const (
	SameSiteDefault SameSite = ""
	SameSiteLax     SameSite = "lax"
	SameSiteStrict  SameSite = "strict"
	SameSiteNone    SameSite = "none"
)

func (s SameSite) MarshalJSON() ([]byte, error) {
	if s == "" {
		return []byte("null"), nil
	}
	return json.Marshal(string(s))
}

func (s *SameSite) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		*s = ""
		return nil
	}

	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	*s = SameSite(str)
	return nil
}

type Cookie struct {
	Domain         string   `json:"domain"`
	ExpirationDate float64  `json:"expirationDate"`
	HostOnly       bool     `json:"hostOnly"`
	HTTPOnly       bool     `json:"httpOnly"`
	Name           string   `json:"name"`
	Path           string   `json:"path"`
	SameSite       SameSite `json:"sameSite"`
	Secure         bool     `json:"secure"`
	Session        bool     `json:"session"`
	StoreID        *string  `json:"storeId,omitempty"`
	Value          string   `json:"value"`
}

func (c *Cookie) UnmarshalJSON(data []byte) error {
	type alias Cookie
	var raw struct {
		alias
		Key    string          `json:"key"`
		MaxAge float64         `json:"maxAge"`
		ExpRaw json.RawMessage `json:"expires"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*c = Cookie(raw.alias)
	if c.Name == "" && raw.Key != "" {
		c.Name = raw.Key
	}
	if c.ExpirationDate == 0 {
		if raw.MaxAge > 0 {
			c.ExpirationDate = float64(time.Now().Unix()) + raw.MaxAge
		} else if len(raw.ExpRaw) > 0 {
			var expStr string
			if err := json.Unmarshal(raw.ExpRaw, &expStr); err == nil {
				if t, err := time.Parse(time.RFC3339Nano, expStr); err == nil {
					c.ExpirationDate = float64(t.UnixNano()) / 1e9
				}
			}
		}
	}
	return nil
}

func (c Cookie) ToHTTPCookie() *http.Cookie {
	hc := &http.Cookie{
		Domain:   c.Domain,
		Name:     c.Name,
		Value:    c.Value,
		Path:     c.Path,
		HttpOnly: c.HTTPOnly,
		Secure:   c.Secure,
	}

	switch c.SameSite {
	case SameSiteStrict:
		hc.SameSite = http.SameSiteStrictMode
	case SameSiteLax:
		hc.SameSite = http.SameSiteLaxMode
	case SameSiteNone:
		hc.SameSite = http.SameSiteNoneMode
	default:
		hc.SameSite = http.SameSiteDefaultMode
	}

	if !c.Session && c.ExpirationDate > 0 {
		sec := int64(c.ExpirationDate)                         // whole seconds
		nsec := int64((c.ExpirationDate - float64(sec)) * 1e9) // fractional part → nanoseconds
		hc.Expires = time.Unix(sec, nsec)
	}

	return hc
}

func (c *Cookie) FromHTTPCookie(hc *http.Cookie) {
	c.Domain = hc.Domain
	c.Name = hc.Name
	c.Value = hc.Value
	c.Path = hc.Path
	c.HTTPOnly = hc.HttpOnly
	c.Secure = hc.Secure
	c.HostOnly = hc.Domain == ""
	c.StoreID = nil

	switch hc.SameSite {
	case http.SameSiteStrictMode:
		c.SameSite = SameSiteStrict
	case http.SameSiteLaxMode:
		c.SameSite = SameSiteLax
	case http.SameSiteNoneMode:
		c.SameSite = SameSiteNone
	default:
		c.SameSite = SameSiteDefault
	}

	switch {
	case hc.MaxAge > 0:
		exp := time.Now().Add(time.Duration(hc.MaxAge) * time.Second)
		c.Session = false
		c.ExpirationDate = float64(exp.UnixNano()) / 1e9
	case hc.MaxAge == 0 && !hc.Expires.IsZero():
		c.Session = false
		c.ExpirationDate = float64(hc.Expires.UnixNano()) / 1e9
	default:
		c.Session = true
		c.ExpirationDate = 0
	}
}

type J2Cookie struct {
	URL     string   `json:"url"`
	Cookies []Cookie `json:"cookies"`
}

// CookieUnion represents cookies in multiple formats.
//
// Supported formats:
//
// 1. Cookie Array
//
//	[{"name": "session", "value": "abc123", "domain": ".zalo.me"}]
//
// 2. J2Cookie Object
//
//	{"url": "https://chat.zalo.me", "cookies": [...]}
type CookieUnion struct {
	cookies  []Cookie
	j2cookie *J2Cookie
}

func NewHTTPCookie(hc []*http.Cookie) CookieUnion {
	cu := CookieUnion{}
	if hc == nil {
		cu.cookies = nil
		cu.j2cookie = nil
		return cu
	}

	cookies := make([]Cookie, len(hc))
	for i, c := range hc {
		var ck Cookie
		ck.FromHTTPCookie(c)
		cookies[i] = ck
	}

	cu.cookies = cookies
	cu.j2cookie = nil
	return cu
}
func NewCookieArray(c []Cookie) CookieUnion { return CookieUnion{cookies: c} }
func NewJ2Cookie(j J2Cookie) CookieUnion    { return CookieUnion{j2cookie: &j} }

func (cu *CookieUnion) IsValid() bool    { return cu.cookies != nil || cu.j2cookie != nil }
func (cu *CookieUnion) IsArray() bool    { return cu.cookies != nil }
func (cu *CookieUnion) IsJ2Cookie() bool { return cu.j2cookie != nil }
func (cu *CookieUnion) GetCookies() []Cookie {
	if cu.cookies != nil {
		return cu.cookies
	}
	if cu.j2cookie != nil {
		return cu.j2cookie.Cookies
	}
	return nil
}

func (cu *CookieUnion) GetHTTPCookies() []*http.Cookie {
	cookies := cu.GetCookies()
	if cookies == nil {
		return nil
	}
	httpCookies := make([]*http.Cookie, len(cookies))
	for i, c := range cookies {
		httpCookies[i] = c.ToHTTPCookie()
	}
	return httpCookies
}

// IsDeleteMarker kiểm tra xem cookie có phải là lệnh xóa cookie cũ của Zalo hay không
// (Set-Cookie mang value "EXPIRED" hoặc thời hạn năm 1970).
func IsDeleteMarker(c Cookie) bool {
	return c.Value == "EXPIRED" || (c.ExpirationDate > 0 && c.ExpirationDate < 86400)
}

func (cu *CookieUnion) BuildCookieJar(u *url.URL, jar http.CookieJar) {
	cookieArr := cu.GetCookies()

	for i := range cookieArr {
		if len(cookieArr[i].Domain) > 0 && cookieArr[i].Domain[0] == '.' {
			cookieArr[i].Domain = cookieArr[i].Domain[1:]
		}
	}

	if jar == nil {
		jar, _ = cookiejar.New(nil)
	}

	domainGroups := make(map[string][]*http.Cookie)
	for _, c := range cookieArr {
		if IsDeleteMarker(c) {
			continue // Bỏ qua cookie xóa để bảo vệ session token
		}
		hc := c.ToHTTPCookie()
		domain := c.Domain
		if domain == "" && u != nil {
			domain = u.Host
		}
		domainGroups[domain] = append(domainGroups[domain], hc)
	}

	for domain, cks := range domainGroups {
		targetURL := u
		if domain != "" {
			if parsed, err := url.Parse("https://" + domain); err == nil {
				targetURL = parsed
			}
		}
		jar.SetCookies(targetURL, cks)
	}
}

func (cu CookieUnion) MarshalJSON() ([]byte, error) {
	switch {
	case cu.cookies != nil && cu.j2cookie != nil:
		return nil, errs.NewZCA("both cookies and j2cookie are set", "CookieUnion.MarshalJSON")
	case cu.cookies != nil:
		return json.Marshal(cu.cookies)
	case cu.j2cookie != nil:
		return json.Marshal(cu.j2cookie)
	default:
		return []byte("null"), nil
	}
}

func (cu *CookieUnion) UnmarshalJSON(b []byte) error {
	trim := bytes.TrimSpace(b)
	if len(trim) == 0 || bytes.Equal(trim, []byte("null")) {
		*cu = CookieUnion{}
		return nil
	}

	if trim[0] == '[' {
		var arr []Cookie
		if err := json.Unmarshal(trim, &arr); err != nil {
			return err
		}
		*cu = CookieUnion{cookies: arr}
		return nil
	}

	var j J2Cookie
	if err := json.Unmarshal(trim, &j); err != nil {
		return err
	}
	*cu = CookieUnion{j2cookie: &j}
	return nil
}
