package api

import (
	"encoding/json"
	"testing"
)

func TestLogoutHTMLRegex(t *testing.T) {
	// 1. Dạng object literal: listSetSidUrls: [...] (Zalo web production)
	objHTML := `<!DOCTYPE html>
<html>
<head>
<script>
var config = {
	listSetSidUrls: ["https://chat.zalo.me/setsid?token=abc", "https://zalo.me/setsid?token=xyz"]
};
</script>
</head>
<body>Logging out...</body>
</html>`

	matches := listSetSidRegex.FindSubmatch([]byte(objHTML))
	if len(matches) < 2 {
		t.Fatalf("expected regex match for listSetSidUrls with ':', got none")
	}

	var urls []string
	if err := json.Unmarshal(matches[1], &urls); err != nil {
		t.Fatalf("failed to unmarshal matched urls: %v", err)
	}

	if len(urls) != 2 {
		t.Fatalf("expected 2 urls, got %d", len(urls))
	}
	if urls[0] != "https://chat.zalo.me/setsid?token=abc" {
		t.Fatalf("unexpected url[0]: %s", urls[0])
	}

	// 2. Dạng gán biến: listSetSidUrls = [...]
	assignHTML := `window.listSetSidUrls = ["https://chat.zalo.me/setsid?token=assign"];`
	matchesAssign := listSetSidRegex.FindSubmatch([]byte(assignHTML))
	if len(matchesAssign) < 2 {
		t.Fatalf("expected regex match for listSetSidUrls with '=', got none")
	}
}
