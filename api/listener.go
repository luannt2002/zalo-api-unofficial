package api

import "github.com/luannt2002/zalo-api-unofficial/listener"

func (a *api) Listener() listener.Listener {
	return a.l
}
