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
	// GetZNoiseRequest chứa danh sách user UID (fids) và group ID (gids).
	GetZNoiseRequest struct {
		FIDs []string `json:"fids,omitempty"`
		GIDs []string `json:"gids,omitempty"`
	}

	// ZNoiseData chứa mảng ánh xạ số âm i64 theo ngữ cảnh voicecall.
	ZNoiseData struct {
		FIDs []int64 `json:"fids"`
		GIDs []int64 `json:"gids"`
	}

	// GetZNoiseResponse là phản hồi từ endpoint /api/znoise (cmd 12390).
	GetZNoiseResponse struct {
		ErrorCode int        `json:"error_code"`
		ErrorMsg  string     `json:"error_message"`
		Data      ZNoiseData `json:"data"`
	}

	GetZNoiseFn = func(ctx context.Context, req GetZNoiseRequest) (*GetZNoiseResponse, error)
)

func (r *GetZNoiseResponse) UnmarshalJSON(b []byte) error {
	type rawResponse struct {
		ErrorCode int        `json:"error_code"`
		ErrorMsg  string     `json:"error_message"`
		Data      ZNoiseData `json:"data"`
		FIDs      []int64    `json:"fids"`
		GIDs      []int64    `json:"gids"`
	}
	var raw rawResponse
	if err := jsonx.UnmarshalLenient(b, &raw); err != nil {
		return err
	}
	r.ErrorCode = raw.ErrorCode
	r.ErrorMsg = raw.ErrorMsg
	if len(raw.Data.FIDs) > 0 || len(raw.Data.GIDs) > 0 {
		r.Data = raw.Data
	} else if len(raw.FIDs) > 0 || len(raw.GIDs) > 0 {
		r.Data = ZNoiseData{
			FIDs: raw.FIDs,
			GIDs: raw.GIDs,
		}
	}
	return nil
}

func (a *api) GetZNoise(ctx context.Context, req GetZNoiseRequest) (*GetZNoiseResponse, error) {
	return a.e.GetZNoise(ctx, req)
}

var getZNoiseFactory = apiFactory[*GetZNoiseResponse, GetZNoiseFn]()(
	func(a *api, sc session.Context, u factoryUtils[*GetZNoiseResponse]) (GetZNoiseFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("profile"), "")
		serviceURL := u.MakeURL(base+"/api/znoise", nil, true)

		return func(ctx context.Context, req GetZNoiseRequest) (*GetZNoiseResponse, error) {
			if len(req.FIDs) == 0 && len(req.GIDs) == 0 {
				return nil, errs.NewZCA("fids and gids cannot both be empty", "api.GetZNoise")
			}

			payload := map[string]any{}
			if len(req.FIDs) > 0 {
				payload["fids"] = req.FIDs
			}
			if len(req.GIDs) > 0 {
				payload["gids"] = req.GIDs
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return nil, errs.WrapZCA("failed to encrypt params", "api.GetZNoise", err)
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
