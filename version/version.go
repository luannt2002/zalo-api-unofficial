package version

import (
	"context"

	"github.com/luannt2002/zalo-api-unofficial/session"
)

const version = "v0.3.0"

func GetVersion() string {
	return version
}

// CheckUpdate is intentionally a no-op.
//
// github.com/luannt2002/zalo-api-unofficial is an internal, replace-only module (require v0.0.0 +
// replace => ../github.com/luannt2002/zalo-api-unofficial) and is NOT published to the Go module proxy,
// so there is no remote registry to query for a newer version. The previous
// implementation made a wasted HTTP call to proxy.golang.org/@latest on every
// login that could never return a meaningful version. The function is kept for
// API compatibility (called from zcago.go and gated by WithCheckUpdate); it now
// returns immediately without any network I/O.
func CheckUpdate(_ context.Context, _ session.Context) {}
