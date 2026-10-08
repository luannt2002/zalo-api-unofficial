package api

import (
	"context"
	"net/http"

	"github.com/luannt2002/zalo-api-unofficial/errs"
	"github.com/luannt2002/zalo-api-unofficial/session"
)

type (
	GetCookieResponse = http.CookieJar
	GetCookieFn       = func(ctx context.Context) (GetCookieResponse, error)
)

func (a *api) GetCookie(ctx context.Context) (GetCookieResponse, error) {
	return a.e.GetCookie(ctx)
}

var getCookieFactory = apiFactory[GetCookieResponse, GetCookieFn]()(
	func(a *api, sc session.Context, u factoryUtils[GetCookieResponse]) (GetCookieFn, error) {
		return func(ctx context.Context) (GetCookieResponse, error) {
			jar := sc.CookieJar()
			if jar == nil {
				return nil, errs.NewZCA("cookie is not available", "api.GetCookie")
			}
			return jar, nil
		}, nil
	},
)
