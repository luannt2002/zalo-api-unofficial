package api

import (
	"context"

	"github.com/luannt2002/zalo-api-unofficial/errs"
	"github.com/luannt2002/zalo-api-unofficial/session"
)

type ConvertGlobalIDDirection string

const (
	ConvertDirectionEncode ConvertGlobalIDDirection = "encode" // UID -> GID
	ConvertDirectionDecode ConvertGlobalIDDirection = "decode" // GID -> UID
)

type (
	ConvertGlobalIDRequest struct {
		IDs       []string                 `json:"ids"`
		Direction ConvertGlobalIDDirection `json:"direction"` // "encode" or "decode"
	}

	ConvertGlobalIDResponse struct {
		Direction ConvertGlobalIDDirection `json:"direction"`
		Mapping   map[string]string        `json:"mapping"`
	}

	ConvertGlobalIDFn = func(ctx context.Context, req ConvertGlobalIDRequest) (*ConvertGlobalIDResponse, error)
)

func (a *api) ConvertGlobalID(ctx context.Context, req ConvertGlobalIDRequest) (*ConvertGlobalIDResponse, error) {
	return a.e.ConvertGlobalID(ctx, req)
}

var convertGlobalIDFactory = apiFactory[*ConvertGlobalIDResponse, ConvertGlobalIDFn]()(
	func(a *api, sc session.Context, u factoryUtils[*ConvertGlobalIDResponse]) (ConvertGlobalIDFn, error) {
		return func(ctx context.Context, req ConvertGlobalIDRequest) (*ConvertGlobalIDResponse, error) {
			if len(req.IDs) == 0 {
				return nil, errs.NewZCA("ids cannot be empty", "api.ConvertGlobalID")
			}

			direction := req.Direction
			if direction == "" {
				direction = ConvertDirectionEncode
			}

			mapping := make(map[string]string)

			switch direction {
			case ConvertDirectionEncode:
				res, err := a.EncryptGlobalID(ctx, EncryptGlobalIDRequest{UIDs: req.IDs})
				if err != nil {
					return nil, err
				}
				for k, v := range res {
					mapping[k] = v
				}
			case ConvertDirectionDecode:
				res, err := a.DecryptGlobalID(ctx, DecryptGlobalIDRequest{GlobalIDs: req.IDs})
				if err != nil {
					return nil, err
				}
				for k, v := range res {
					mapping[k] = v
				}
			default:
				return nil, errs.NewZCA("invalid direction: must be encode or decode", "api.ConvertGlobalID")
			}

			return &ConvertGlobalIDResponse{
				Direction: direction,
				Mapping:   mapping,
			}, nil
		}, nil
	},
)

