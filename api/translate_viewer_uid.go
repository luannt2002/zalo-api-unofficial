package api

import (
	"context"

	"github.com/luannt2002/zalo-api-unofficial/errs"
	"github.com/luannt2002/zalo-api-unofficial/pkg/gid"
	"github.com/luannt2002/zalo-api-unofficial/session"
)

type (
	TranslateViewerUIDRequest struct {
		UIDs          []string `json:"uids"`
		SourceSend2me string   `json:"sourceSend2me"`
		TargetSend2me string   `json:"targetSend2me"`
	}

	TranslateViewerUIDResponse struct {
		Mask    uint64            `json:"mask"`
		Mapping map[string]string `json:"mapping"`
	}

	TranslateViewerUIDFn = func(ctx context.Context, req TranslateViewerUIDRequest) (*TranslateViewerUIDResponse, error)
)

func (a *api) TranslateViewerUID(ctx context.Context, req TranslateViewerUIDRequest) (*TranslateViewerUIDResponse, error) {
	return a.e.TranslateViewerUID(ctx, req)
}

var translateViewerUIDFactory = apiFactory[*TranslateViewerUIDResponse, TranslateViewerUIDFn]()(
	func(a *api, sc session.Context, u factoryUtils[*TranslateViewerUIDResponse]) (TranslateViewerUIDFn, error) {
		return func(ctx context.Context, req TranslateViewerUIDRequest) (*TranslateViewerUIDResponse, error) {
			if len(req.UIDs) == 0 {
				return nil, errs.NewZCA("uids cannot be empty", "api.TranslateViewerUID")
			}

			src := req.SourceSend2me
			if src == "" {
				src = sc.Send2meID()
			}
			if src == "" {
				return nil, errs.NewZCA("source send2me_id is required", "api.TranslateViewerUID")
			}
			if req.TargetSend2me == "" {
				return nil, errs.NewZCA("target send2me_id is required", "api.TranslateViewerUID")
			}

			mask, err := gid.MaskFromSend2me(src, req.TargetSend2me)
			if err != nil {
				return nil, errs.WrapZCA("failed to compute pair mask", "api.TranslateViewerUID", err)
			}

			mapping := make(map[string]string, len(req.UIDs))
			for _, uid := range req.UIDs {
				trans, err := gid.TranslateViewerUID(uid, src, req.TargetSend2me)
				if err != nil {
					return nil, errs.WrapZCA("failed to translate uid", "api.TranslateViewerUID", err)
				}
				mapping[uid] = trans
			}

			return &TranslateViewerUIDResponse{
				Mask:    mask,
				Mapping: mapping,
			}, nil
		}, nil
	},
)
