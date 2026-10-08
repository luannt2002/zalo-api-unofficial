package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strconv"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	"github.com/luannt2002/zalo-api-unofficial/config"
	"github.com/luannt2002/zalo-api-unofficial/errs"
	"github.com/luannt2002/zalo-api-unofficial/internal/httpx"
	"github.com/luannt2002/zalo-api-unofficial/internal/jsonx"
	"github.com/luannt2002/zalo-api-unofficial/model"
	"github.com/luannt2002/zalo-api-unofficial/session"
)

type (
	fileData struct {
		FileName  string `json:"fileName"`
		TotalSize int64  `json:"totalSize"`

		Width  *int `json:"width,omitempty"`
		Height *int `json:"height,omitempty"`
	}
	attachmentParams struct {
		ToID       *string `json:"toid,omitempty"`
		Grid       *string `json:"grid,omitempty"`
		TotalChunk int     `json:"totalChunk"`
		FileName   string  `json:"fileName"`
		ClientID   int64   `json:"clientId"`
		TotalSize  int64   `json:"totalSize"`
		IMEI       string  `json:"imei"`
		IsE2EE     int     `json:"isE2EE"`
		JXL        int     `json:"jxl"`
		ChunkID    int     `json:"chunkId"`
	}
	fileAttachmentData struct {
		FilePath     string                 `json:"filePath"`
		FileType     model.FileType         `json:"fileType"` // "video" | "others"
		ChunkContent []httpx.FormData       `json:"chunkContent"`
		FileData     fileData               `json:"fileData"`
		Params       attachmentParams       `json:"params"`
		Source       model.AttachmentSource `json:"source"`
	}

	rawResponse struct {
		Finished     bool `json:"finished"`
		ClientFileID int  `json:"clientFileId"`
		ChunkID      int  `json:"chunkId"`

		FileID  *string `json:"fileId,omitempty"`
		PhotoID *string `json:"photoId,omitempty"` // ThreadTypeGroup return int

		HDURL     *string `json:"hdUrl,omitempty"`
		NormalURL *string `json:"normalUrl,omitempty"`
		ThumbURL  *string `json:"thumbUrl,omitempty"`
	}

	UploadAttachment struct {
		Finished     bool           `json:"finished"`
		ClientFileID int            `json:"clientFileId"`
		ChunkID      int            `json:"chunkId"`
		FileType     model.FileType `json:"fileType"` // "image" | "video" | "others"
		TotalSize    int64          `json:"totalSize"`

		Image *UploadImageInfo `json:"-"`
		File  *UploadFileInfo  `json:"-"`
	}
	UploadImageInfo struct {
		PhotoID   string `json:"photoId"`
		HDURL     string `json:"hdUrl"`
		ThumbURL  string `json:"thumbUrl"`
		NormalURL string `json:"normalUrl"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
		HDSize    int64  `json:"hdSize"`
	}
	UploadFileInfo struct {
		FileID   string `json:"fileId"`
		FileURL  string `json:"fileUrl"`
		FileName string `json:"fileName"`
		Checksum string `json:"checksum"`
	}

	UploadAttachmentResponse = []UploadAttachment
	UploadAttachmentFn       = func(ctx context.Context, threadID string, threadType model.ThreadType, sources ...model.AttachmentSource) (UploadAttachmentResponse, error)
)

func (a *api) UploadAttachment(ctx context.Context, threadID string, threadType model.ThreadType, sources ...model.AttachmentSource) (UploadAttachmentResponse, error) {
	return a.e.UploadAttachment(ctx, threadID, threadType, sources...)
}

var uploadAttachmentFactory = apiFactory[UploadAttachmentResponse, UploadAttachmentFn]()(
	func(a *api, sc session.Context, u factoryUtils[UploadAttachmentResponse]) (UploadAttachmentFn, error) {
		base := jsonx.FirstOr(sc.GetZpwService("file"), "")
		serviceURL := u.MakeURL(base+"/api", nil, false)
		shareFile := sc.Settings().Features.ShareFile

		pathMap := map[model.FileType]string{
			model.FileTypeImage: "/photo_original/upload",
			model.FileTypeVideo: "/asyncfile/upload",
			model.FileTypeOther: "/asyncfile/upload",
		}

		isExceedMaxFile := func(totalFile int) bool {
			return totalFile > shareFile.MaxFile
		}

		isExceedMaxFileSize := func(totalSize int64) bool {
			return totalSize > shareFile.MaxSizeShareFileV3*1024*1024
		}

		isValidExtension := func(ext string) bool {
			return slices.Index(shareFile.RestrictedExtFile, ext) == -1
		}

		return func(ctx context.Context, threadID string, threadType model.ThreadType, sources ...model.AttachmentSource) (UploadAttachmentResponse, error) {
			if len(sources) == 0 {
				return nil, errs.ErrSourceEmpty
			}
			if isExceedMaxFile(len(sources)) {
				return nil, errs.ErrExceedMaxFile
			}

			clientID := time.Now().UnixMilli()
			chunkSize := shareFile.ChunkSizeFile
			isGroup := threadType == model.ThreadTypeGroup

			attachments := make([]fileAttachmentData, 0, len(sources))
			for _, source := range sources {
				var (
					reader       io.Reader
					closer       io.Closer
					fileName     string
					fileMetadata model.AttachmentMetadata
				)
				cleanup := func() {
					if closer != nil {
						_ = closer.Close()
					}
				}

				if f := source.String(); f != "" {
					r, err := os.Open(f)
					if err != nil {
						return nil, errs.WrapZCA("failed to read file", "api.UploadAttachment", err)
					}

					reader = r
					closer = r

					fileMetadata, fileName, err = sc.GetImageMetadata(f)
					if err != nil {
						cleanup()
						return nil, err
					}
				} else if f := source.Object(); f != nil {
					reader, fileName, fileMetadata = f.Data, f.Filename, f.Metadata
				}

				ext := source.GetExtension()
				if !isValidExtension(ext) {
					cleanup()
					return nil, errs.ErrInvalidExtension
				}
				if isExceedMaxFileSize(fileMetadata.Size) {
					cleanup()
					return nil, errs.ErrExceedMaxFileSize
				}

				forms, err := httpx.BuildFormData(
					"chunkContent", reader,
					httpx.WithContentType("application/octet-stream"),
					httpx.WithFileName(fileName),
					httpx.WithChunkSize(chunkSize),
				)
				cleanup()

				if err != nil || len(forms) == 0 {
					return nil, errs.WrapZCA("failed to build form data", "api.UploadAttachment", err)
				}

				chunkContent := make([]httpx.FormData, len(forms))
				for i, formData := range forms {
					if formData != nil {
						chunkContent[i] = *formData
					}
				}

				params := attachmentParams{
					FileName:   fileName,
					ClientID:   clientID,
					TotalSize:  fileMetadata.Size,
					IMEI:       sc.IMEI(),
					IsE2EE:     0,
					JXL:        0,
					ChunkID:    1,
					TotalChunk: int((fileMetadata.Size + chunkSize - 1) / chunkSize),
				}
				clientID++

				if isGroup {
					params.Grid = &threadID
				} else {
					params.ToID = &threadID
				}

				var fileType model.FileType
				switch ext {
				case "jpg", "jpeg", "png", "webp":
					fileType = model.FileTypeImage
				case "mp4":
					fileType = model.FileTypeVideo
				default:
					fileType = model.FileTypeOther
				}

				attachments = append(attachments, fileAttachmentData{
					FilePath:     fileName,
					FileType:     fileType,
					ChunkContent: chunkContent,
					FileData: fileData{
						FileName:  fileName,
						TotalSize: fileMetadata.Size,
						Width:     &fileMetadata.Width,
						Height:    &fileMetadata.Height,
					},
					Params: params,
					Source: source,
				})
			}

			typeParam := "2"
			if isGroup {
				typeParam = "11"
			}

			// reqURL là biến LOCAL mỗi lần gọi — KHÔNG mutate serviceURL (closure dùng
			// chung ở factory). Trước đây `serviceURL += "/message"` tích lũy qua từng
			// lần upload: lần 1 ".../message" (200), lần 2 ".../message/message" (404),
			// lần 3 thêm nữa... ⇒ chỉ ảnh ĐẦU TIÊN mỗi tiến trình gửi được.
			reqURL := serviceURL
			if isGroup {
				reqURL += "/group"
			} else {
				reqURL += "/message"
			}

			var (
				mu      sync.Mutex
				g, gctx = errgroup.WithContext(ctx)
				results = make([]UploadAttachment, 0, len(attachments))

				// Async (video/file) uploads complete via a websocket
				// "file_done" event. Each registered callback is paired with a
				// once-guarded cancel so the WaitGroup is always released:
				// either by the event, by the CallbacksMap TTL eviction, or by
				// an explicit cancel below. This guarantees cbWG.Wait() can
				// never hang forever and no goroutine leaks.
				cbWG      sync.WaitGroup
				cbErrMu   sync.Mutex
				cbErr     error
				pendingMu sync.Mutex
				pending   = make(map[string]func(key string))
			)

			cancelPending := func() {
				pendingMu.Lock()
				defer pendingMu.Unlock()
				for id, cancel := range pending {
					sc.UploadCallback().Delete(id)
					cancel(id)
				}
				clear(pending)
			}

			for ai := range attachments {
				a := attachments[ai]
				baseParams := a.Params

				for ci := 0; ci < baseParams.TotalChunk; ci++ {
					chunk := ci

					g.Go(func() error {
						p := baseParams
						p.ChunkID = baseParams.ChunkID + chunk

						enc, err := u.EncodeAES(jsonx.Stringify(p))
						if err != nil {
							return errs.WrapZCA("failed to encrypt params", "api.UploadAttachment", err)
						}

						url := u.MakeURL(
							reqURL+pathMap[a.FileType],
							map[string]any{"type": typeParam, "params": enc},
							true,
						)
						reqCtx := gctx
						resp, err := u.Request(reqCtx, url, &httpx.RequestOptions{
							Method:  http.MethodPost,
							Headers: a.ChunkContent[chunk].Header,
							Body:    a.ChunkContent[chunk].Body,
						})
						if err != nil {
							return err
						}

						data, err := resolveResponse[rawResponse](sc, resp, true)
						_ = resp.Body.Close()
						if err != nil {
							return err
						}

						hasFileID := data.FileID != nil && *data.FileID != "-1"
						hasPhotoID := data.PhotoID != nil && *data.PhotoID != "-1"

						if hasFileID || hasPhotoID {
							switch a.FileType {
							case model.FileTypeVideo, model.FileTypeOther:
								if data.FileID == nil {
									return errs.NewZCA("missing fileId in upload response", "api.UploadAttachment")
								}
								fileID := *data.FileID

								cbWG.Add(1)
								var once sync.Once
								uploadCallback := func(wsData model.UploadAttachment) {
									once.Do(func() {
										defer cbWG.Done()
										checksum := ""
										if c := a.Source.GetLargeFileMD5(); c != nil {
											checksum = c.Checksum
										}

										result := UploadAttachment{
											FileType:     a.FileType,
											Finished:     data.Finished,
											ClientFileID: data.ClientFileID,
											ChunkID:      data.ChunkID,
											TotalSize:    a.FileData.TotalSize,
											File: &UploadFileInfo{
												FileID:   fileID,
												FileURL:  wsData.FileURL,
												FileName: a.FileData.FileName,
												Checksum: checksum,
											},
										}

										mu.Lock()
										results = append(results, result)
										mu.Unlock()
									})
								}
								// Fired by CallbacksMap TTL eviction (or by
								// cancelPending) when the file_done event never
								// arrives; releases the WaitGroup exactly once.
								onExpire := func(key string) {
									once.Do(func() {
										defer cbWG.Done()
										cbErrMu.Lock()
										if cbErr == nil {
											cbErr = errs.NewZCA(
												fmt.Sprintf("timed out waiting for file_done event (fileId=%s)", key),
												"api.UploadAttachment",
											)
										}
										cbErrMu.Unlock()
									})
								}

								pendingMu.Lock()
								pending[fileID] = onExpire
								pendingMu.Unlock()

								sc.UploadCallback().Set(fileID, uploadCallback, onExpire, 0)

							case model.FileTypeImage:
								if data.PhotoID == nil || data.NormalURL == nil || data.HDURL == nil || data.ThumbURL == nil {
									return errs.NewZCA("missing photo fields in upload response", "api.UploadAttachment")
								}
								// Width/Height được set khi đọc header ảnh trước upload; nil ⇒ không đọc được
								// kích thước ⇒ deref *a.FileData.Width/Height sẽ panic. Chặn sớm, báo lỗi rõ.
								if a.FileData.Width == nil || a.FileData.Height == nil {
									return errs.NewZCA("missing image dimensions before upload", "api.UploadAttachment")
								}
								result := UploadAttachment{
									FileType:     model.FileTypeImage,
									Finished:     data.Finished,
									ClientFileID: data.ClientFileID,
									ChunkID:      data.ChunkID,
									TotalSize:    a.FileData.TotalSize,

									Image: &UploadImageInfo{
										HDSize:    a.FileData.TotalSize,
										PhotoID:   *data.PhotoID,
										Width:     *a.FileData.Width,
										Height:    *a.FileData.Height,
										NormalURL: *data.NormalURL,
										HDURL:     *data.HDURL,
										ThumbURL:  *data.ThumbURL,
									},
								}

								mu.Lock()
								results = append(results, result)
								mu.Unlock()
							}
						}
						return nil
					})
				}
			}

			if err := g.Wait(); err != nil {
				// Release any callbacks already registered before the failure
				// so the WaitGroup (and the session callback map) never holds
				// orphaned entries.
				cancelPending()
				cbWG.Wait()
				return nil, err
			}

			// Wait for the websocket completion events, but never forever:
			// the CallbacksMap TTL eviction releases cbWG on its own, and the
			// caller's ctx plus a safety-net timeout (TTL + grace) bound the
			// wait here. On early exit, cancelPending releases every pending
			// entry so the waiter goroutine below always terminates.
			cbDone := make(chan struct{})
			go func() {
				cbWG.Wait()
				close(cbDone)
			}()

			waitTimeout := config.DefaultUploadCallbackTTL + 30*time.Second
			timer := time.NewTimer(waitTimeout)
			defer timer.Stop()

			select {
			case <-cbDone:
			case <-ctx.Done():
				cancelPending()
				<-cbDone
				return nil, errs.WrapZCA("context cancelled while waiting for upload completion", "api.UploadAttachment", ctx.Err())
			case <-timer.C:
				cancelPending()
				<-cbDone
				return nil, errs.NewZCA("timed out waiting for upload completion", "api.UploadAttachment")
			}

			cbErrMu.Lock()
			err := cbErr
			cbErrMu.Unlock()
			if err != nil {
				return nil, err
			}

			return results, nil
		}, nil
	},
)

func (r *rawResponse) UnmarshalJSON(data []byte) error {
	type alias rawResponse
	aux := &struct {
		*alias

		Finished any `json:"finished"`
		PhotoID  any `json:"photoId"`
	}{
		alias: (*alias)(r),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	switch v := aux.PhotoID.(type) {
	case float64:
		s := strconv.FormatInt(int64(v), 10)
		r.PhotoID = &s
	case string:
		r.PhotoID = &v
	case json.Number:
		s := v.String()
		r.PhotoID = &s
	case nil:
	default:
		s := fmt.Sprintf("%v", v)
		r.PhotoID = &s
	}

	switch v := aux.Finished.(type) {
	case bool:
		r.Finished = v
	case json.Number:
		n, err := v.Int64()
		r.Finished = err == nil && n != 0
	case float64:
		r.Finished = v != 0
	default:
		r.Finished = false
	}

	return nil
}
