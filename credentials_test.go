package zcago

import (
	"encoding/json"
	"fmt"
	"net/http/cookiejar"
	"net/url"
	"testing"

	"github.com/luannt2002/zalo-api-unofficial/internal/testenv"
)

func TestCookieToughCookieFormat(t *testing.T) {
	// Giả lập export từ tough-cookie / zca-js có key và maxAge
	data := `[
		{"key":"zpdid","value":"v_zpdid","domain":"id.zalo.me","path":"/","secure":true,"httpOnly":true},
		{"key":"zpw_sek","value":"v_zpw_sek","domain":"chat.zalo.me","path":"/","secure":true,"httpOnly":true},
		{"name":"regular","value":"v_regular","domain":"zalo.me","path":"/","secure":true,"httpOnly":false}
	]`

	var cu CookieUnion
	if err := json.Unmarshal([]byte(data), &cu); err != nil {
		t.Fatalf("failed to unmarshal CookieUnion: %v", err)
	}

	cookies := cu.GetCookies()
	if len(cookies) != 3 {
		t.Fatalf("expected 3 cookies, got %d", len(cookies))
	}

	if cookies[0].Name != "zpdid" {
		t.Fatalf("expected cookies[0].Name == 'zpdid', got %q", cookies[0].Name)
	}
	if cookies[1].Name != "zpw_sek" {
		t.Fatalf("expected cookies[1].Name == 'zpw_sek', got %q", cookies[1].Name)
	}
	if cookies[2].Name != "regular" {
		t.Fatalf("expected cookies[2].Name == 'regular', got %q", cookies[2].Name)
	}

	// Kiểm tra BuildCookieJar phân chia domain chuẩn
	jar, _ := cookiejar.New(nil)
	defaultURL, _ := url.Parse("https://chat.zalo.me")
	cu.BuildCookieJar(defaultURL, jar)

	uChat, _ := url.Parse("https://chat.zalo.me/api")
	chatCookies := jar.Cookies(uChat)
	if len(chatCookies) < 2 {
		t.Fatalf("expected at least 2 cookies for chat.zalo.me (zpw_sek + regular), got %d", len(chatCookies))
	}

	uID, _ := url.Parse("https://id.zalo.me/api")
	idCookies := jar.Cookies(uID)
	if len(idCookies) < 2 {
		t.Fatalf("expected at least 2 cookies for id.zalo.me (zpdid + regular), got %d", len(idCookies))
	}
}

func TestCredentialsUID9(t *testing.T) {
	env := testenv.Load(t)
	expectedUID9 := env.UID9
	if expectedUID9 == "" {
		t.Skip("TEST_ZALO_UID9 not set in environment")
	}

	rawJSON := fmt.Sprintf(`{
		"imei": "test-imei",
		"userAgent": "test-ua",
		"cookie": [
			{"key":"zpw_sek","value":"pRk2.%s.a0.secretToken","domain":"chat.zalo.me"}
		]
	}`, expectedUID9)

	var cred Credentials
	if err := json.Unmarshal([]byte(rawJSON), &cred); err != nil {
		t.Fatalf("failed to unmarshal credentials: %v", err)
	}

	if got := cred.UID9(); got != expectedUID9 {
		t.Fatalf("cred.UID9() = %q, expected %q", got, expectedUID9)
	}
}

func TestDeleteMarkerFiltering(t *testing.T) {
	// Kiểm tra nhận diện delete marker
	del1 := Cookie{Value: "EXPIRED"}
	del2 := Cookie{ExpirationDate: 10} // năm 1970
	valid := Cookie{Value: "valid_secret_token", ExpirationDate: 1893456000} // tương lai

	if !IsDeleteMarker(del1) {
		t.Fatal("expected del1 to be a delete marker")
	}
	if !IsDeleteMarker(del2) {
		t.Fatal("expected del2 to be a delete marker")
	}
	if IsDeleteMarker(valid) {
		t.Fatal("expected valid cookie NOT to be a delete marker")
	}

	// Kiểm tra BuildCookieJar không nạp delete markers
	data := `[
		{"key":"zpw_sek","value":"EXPIRED","domain":"chat.zalo.me"},
		{"key":"zpsid","value":"valid_session","domain":"chat.zalo.me"}
	]`
	var cu CookieUnion
	if err := json.Unmarshal([]byte(data), &cu); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	jar, _ := cookiejar.New(nil)
	u, _ := url.Parse("https://chat.zalo.me")
	cu.BuildCookieJar(u, jar)

	cookies := jar.Cookies(u)
	if len(cookies) != 1 || cookies[0].Name != "zpsid" {
		t.Fatalf("expected only zpsid to survive, got %v", cookies)
	}
}



