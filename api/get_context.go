package api

import (
	"github.com/luannt2002/zalo-api-unofficial/errs"
	"github.com/luannt2002/zalo-api-unofficial/session"
)

func (a *api) GetContext() (session.Context, error) {
	if a.sc == nil {
		return nil, errs.NewZCA("API context is not initialized", "api.GetContext")
	}

	return a.sc, nil
}
