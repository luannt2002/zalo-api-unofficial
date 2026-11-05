package session

import (
	"fmt"
	"testing"
	"time"

	"github.com/luannt2002/zalo-api-unofficial/internal/testenv"
	"github.com/luannt2002/zalo-api-unofficial/pkg/gid"
)

func TestInspectSession(t *testing.T) {
	env := testenv.Load(t)

	sessionJSON := fmt.Sprintf(`{
		"uid": "%s",
		"imei": "test-imei-12345",
		"userAgent": "Mozilla/5.0 Test",
		"cookies": [
			{
				"key": "zpw_sek",
				"value": "tag4.%s.selector.secret",
				"domain": "chat.zalo.me",
				"expires": "2027-01-01T00:00:00.000Z"
			},
			{
				"key": "zpsid",
				"value": "EXPIRED",
				"domain": "zalo.me",
				"expires": "1970-01-01T00:00:00.000Z"
			}
		]
	}`, env.UIDA, env.UID9)

	diag, err := InspectSession([]byte(sessionJSON))
	if err != nil {
		t.Fatalf("InspectSession failed: %v", err)
	}

	if diag.UID9 != env.UID9 {
		t.Fatalf("expected UID9 %s, got: %s", env.UID9, diag.UID9)
	}
	if !diag.HasDeleteMarkers {
		t.Fatalf("expected HasDeleteMarkers to be true for EXPIRED cookie")
	}
	if len(diag.DeleteMarkerKeys) != 1 || diag.DeleteMarkerKeys[0] != "zpsid@zalo.me" {
		t.Fatalf("unexpected DeleteMarkerKeys: %v", diag.DeleteMarkerKeys)
	}
	if diag.CookieCount != 2 {
		t.Fatalf("expected 2 cookies, got: %d", diag.CookieCount)
	}
	if diag.ViewerCreatedAt == nil || diag.ViewerCreatedAt.Before(gid.SnowflakeEpoch) {
		t.Fatalf("expected valid ViewerCreatedAt after epoch, got: %v", diag.ViewerCreatedAt)
	}
	if diag.ViewerCreatedAt.After(time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("ViewerCreatedAt unreasonably far in future: %v", diag.ViewerCreatedAt)
	}
}

func TestClassifyCookieChanges(t *testing.T) {
	c := CookieRecord{Key: "fixture", Value: "one", Domain: "example.test", Path: "/"}

	// 1. Same cookie -> valueChanged = 0
	r1 := ClassifyCookieChanges([]CookieRecord{c}, []CookieRecord{c})
	if r1.ValueChanged != 0 || r1.MetadataChanged != 0 || r1.Added != 0 || r1.Removed != 0 {
		t.Fatalf("expected all 0, got: %+v", r1)
	}

	// 2. Value changed -> valueChanged = 1
	c2 := c
	c2.Value = "two"
	r2 := ClassifyCookieChanges([]CookieRecord{c}, []CookieRecord{c2})
	if r2.ValueChanged != 1 {
		t.Fatalf("expected ValueChanged=1, got: %+v", r2)
	}

	// 3. Metadata changed (expires) -> metadataChanged = 1
	c3 := c
	c3.Expires = "2030-01-01T00:00:00.000Z"
	r3 := ClassifyCookieChanges([]CookieRecord{c}, []CookieRecord{c3})
	if r3.MetadataChanged != 1 {
		t.Fatalf("expected MetadataChanged=1, got: %+v", r3)
	}

	// 4. Added -> added = 1
	r4 := ClassifyCookieChanges([]CookieRecord{}, []CookieRecord{c})
	if r4.Added != 1 {
		t.Fatalf("expected Added=1, got: %+v", r4)
	}

	// 5. Removed -> removed = 1
	r5 := ClassifyCookieChanges([]CookieRecord{c}, []CookieRecord{})
	if r5.Removed != 1 {
		t.Fatalf("expected Removed=1, got: %+v", r5)
	}
}


