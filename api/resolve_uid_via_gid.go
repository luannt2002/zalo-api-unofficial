package api

import (
	"context"

	"github.com/luannt2002/zalo-api-unofficial/errs"
)

// ResolveUIDViaGID giải mã danh sách GID (nhận từ bot/account khác) sang UID cục bộ dưới góc nhìn của viewer hiện tại.
func (a *api) ResolveUIDViaGID(ctx context.Context, globalIDs ...string) (map[string]string, error) {
	if len(globalIDs) == 0 {
		return nil, errs.NewZCA("missing global IDs", "api.ResolveUIDViaGID")
	}

	res, err := a.DecryptGlobalID(ctx, DecryptGlobalIDRequest{GlobalIDs: globalIDs})
	if err != nil {
		return nil, err
	}

	return map[string]string(res), nil
}
