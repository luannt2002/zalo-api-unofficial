package session_test

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"testing"

	"github.com/luannt2002/zalo-api-unofficial/internal/testenv"
	"github.com/luannt2002/zalo-api-unofficial/session"
)

func TestExtractUID9(t *testing.T) {
	env := testenv.Load(t)
	expectedUID9 := env.UID9
	if expectedUID9 == "" {
		t.Skip("TEST_ZALO_UID9 not set in environment")
	}

	// 1. Valid zpw_sek format (tag4.UID-9.a0.secret)
	zpwSek := fmt.Sprintf("pRk2.%s.a0.LVfhhXdbYGwQ999SECRET", expectedUID9)
	if got := session.ExtractUID9(zpwSek); got != expectedUID9 {
		t.Fatalf("ExtractUID9(zpw_sek) = %q, expected %q", got, expectedUID9)
	}

	// 2. Valid zpsid format (tag4.UID-9.gen.secret)
	zpsid := fmt.Sprintf("eyHL.%s.1099.03BNnKSS6b888SECRET", expectedUID9)
	if got := session.ExtractUID9(zpsid); got != expectedUID9 {
		t.Fatalf("ExtractUID9(zpsid) = %q, expected %q", got, expectedUID9)
	}

	// 3. Invalid formats should return empty string
	invalidCases := []string{
		"",
		"random_cookie_value",
		"tag.non_digit.a0.secret",
		"tag.123.a0.secret", // too short (< 6 digits)
		"tag.1234567890123456789.a0.secret", // too long (> 12 digits)
	}
	for _, tc := range invalidCases {
		if got := session.ExtractUID9(tc); got != "" {
			t.Fatalf("ExtractUID9(%q) = %q, expected empty string", tc, got)
		}
	}
}

func TestExtractUID9FromJar(t *testing.T) {
	env := testenv.Load(t)
	expectedUID9 := env.UID9
	if expectedUID9 == "" {
		t.Skip("TEST_ZALO_UID9 not set in environment")
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("failed to create cookiejar: %v", err)
	}

	chatURL, _ := url.Parse("https://chat.zalo.me")
	jar.SetCookies(chatURL, []*http.Cookie{
		{
			Name:  "zpw_sek",
			Value: fmt.Sprintf("pRk2.%s.a0.LVfhhXdbYGwQ999SECRET", expectedUID9),
		},
	})

	if got := session.ExtractUID9FromJar(jar); got != expectedUID9 {
		t.Fatalf("ExtractUID9FromJar = %q, expected %q", got, expectedUID9)
	}

	// Empty / nil jar
	if got := session.ExtractUID9FromJar(nil); got != "" {
		t.Fatalf("ExtractUID9FromJar(nil) = %q, expected empty string", got)
	}
}

func TestContextUID9(t *testing.T) {
	env := testenv.Load(t)
	expectedUID9 := env.UID9
	if expectedUID9 == "" {
		t.Skip("TEST_ZALO_UID9 not set in environment")
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("failed to create cookiejar: %v", err)
	}
	chatURL, _ := url.Parse("https://chat.zalo.me")
	jar.SetCookies(chatURL, []*http.Cookie{
		{
			Name:  "zpw_sek",
			Value: fmt.Sprintf("pRk2.%s.a0.LVfhhXdbYGwQ999SECRET", expectedUID9),
		},
	})

	sc := session.NewContext()
	sc.SealLogin(session.Seal{
		UID: env.UIDA,
		Jar: jar,
	})

	if got := sc.UID9(); got != expectedUID9 {
		t.Fatalf("sc.UID9() = %q, expected %q", got, expectedUID9)
	}
}

