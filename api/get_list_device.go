package api

import (
	"context"
	"net/http"

	"github.com/luannt2002/zalo-api-unofficial/errs"
	"github.com/luannt2002/zalo-api-unofficial/internal/httpx"
	"github.com/luannt2002/zalo-api-unofficial/internal/jsonx"
	"github.com/luannt2002/zalo-api-unofficial/session"
)

type (
	DeviceLinkedInfo struct {
		MasterID     string `json:"masterId"`
		EncIdentity  string `json:"encIdentity"`
		LastUpdateTS int64  `json:"lastUpdateTs"`
		EncSignature string `json:"encSignature"`
		Companions   []any  `json:"companions"`
	}
	GetListDeviceResponse struct {
		Devices DeviceLinkedInfo `json:"devices"`
	}
	GetListDeviceFn = func(ctx context.Context) (*GetListDeviceResponse, error)
)

func (a *api) GetListDevice(ctx context.Context) (*GetListDeviceResponse, error) {
	return a.e.GetListDevice(ctx)
}

var getListDeviceFactory = apiFactory[*GetListDeviceResponse, GetListDeviceFn]()(
	func(a *api, sc session.Context, u factoryUtils[*GetListDeviceResponse]) (GetListDeviceFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("aext"), "")
		serviceURL := u.MakeURL(base+"/api/devices/linked", nil, true)

		return func(ctx context.Context) (*GetListDeviceResponse, error) {
			payload := map[string]any{
				"imei": sc.IMEI(),
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.GetListDevice", err)
			}

			url := u.MakeURL(serviceURL, map[string]any{"params": enc}, true)
			resp, err := u.Request(ctx, url, &httpx.RequestOptions{Method: http.MethodGet})
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()

			return u.Resolve(resp, true)
		}, nil
	},
)
