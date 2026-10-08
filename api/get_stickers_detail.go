package api

import (
	"context"
	"net/http"
	"sync"

	"github.com/luannt2002/zalo-api-unofficial/errs"
	"github.com/luannt2002/zalo-api-unofficial/internal/httpx"
	"github.com/luannt2002/zalo-api-unofficial/internal/jsonx"
	"github.com/luannt2002/zalo-api-unofficial/model"
	"github.com/luannt2002/zalo-api-unofficial/session"
)

type (
	GetStickersDetailResponse []model.StickerDetail
	GetStickersDetailFn       = func(ctx context.Context, stickerID ...int) (*GetStickersDetailResponse, error)
)

func (a *api) GetStickersDetail(ctx context.Context, stickerID ...int) (*GetStickersDetailResponse, error) {
	return a.e.GetStickersDetail(ctx, stickerID...)
}

var getStickersDetailFactory = apiFactory[*GetStickersDetailResponse, GetStickersDetailFn]()(
	func(a *api, sc session.Context, u factoryUtils[*GetStickersDetailResponse]) (GetStickersDetailFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("sticker"), "")
		serviceURL := u.MakeURL(base+"/api/message/sticker/sticker_detail", nil, true)

		fetchDetail := func(ctx context.Context, stickerID int) (model.StickerDetail, error) {
			var zero model.StickerDetail

			payload := map[string]any{
				"sid": stickerID,
			}

			enc, err := u.EncodeAES(jsonx.Stringify(payload))
			if err != nil {
				return zero, errs.WrapZCA("failed to encrypt params", "api.GetStickersDetail", err)
			}

			url := u.MakeURL(serviceURL, map[string]any{"params": enc}, true)
			resp, err := u.Request(ctx, url, &httpx.RequestOptions{Method: http.MethodGet})
			if err != nil {
				return zero, err
			}
			defer resp.Body.Close()

			return resolveResponse[model.StickerDetail](sc, resp, true)
		}

		return func(ctx context.Context, stickerID ...int) (*GetStickersDetailResponse, error) {
			if len(stickerID) == 0 {
				return nil, errs.NewZCA("missing sticker id", "api.GetStickersDetail")
			}

			results := make([]*model.StickerDetail, len(stickerID))
			var wg sync.WaitGroup

			for i, sid := range stickerID {
				wg.Add(1)
				go func(idx, id int) {
					defer wg.Done()
					detail, err := fetchDetail(ctx, id)
					if err != nil {
						return
					}
					results[idx] = &detail
				}(i, sid)
			}

			wg.Wait()

			stickers := make(GetStickersDetailResponse, 0, len(stickerID))
			for _, r := range results {
				if r != nil {
					stickers = append(stickers, *r)
				}
			}

			return &stickers, nil
		}, nil
	},
)
