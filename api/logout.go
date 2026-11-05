package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"github.com/luannt2002/zalo-api-unofficial/errs"
	"github.com/luannt2002/zalo-api-unofficial/internal/httpx"
	"github.com/luannt2002/zalo-api-unofficial/session"
)

type (
	LogoutResponse struct {
		Status bool `json:"status"`
	}

	LogoutFn = func(ctx context.Context) (*LogoutResponse, error)
)

var listSetSidRegex = regexp.MustCompile(`listSetSidUrls\s*[:=]\s*(\[[^\]]*\])`)

func (a *api) Logout(ctx context.Context) (*LogoutResponse, error) {
	return a.e.Logout(ctx)
}

var logoutFactory = apiFactory[any, LogoutFn]()(
	func(a *api, sc session.Context, u factoryUtils[any]) (LogoutFn, error) {
		const (
			logoutTokenURL = "https://id.zalo.me/jr/logout-token"
			logoutURL      = "https://id.zalo.me/account/logout"
		)

		return func(ctx context.Context) (*LogoutResponse, error) {
			// 1. Lấy logout token từ id.zalo.me
			tokenForm := url.Values{}
			tokenForm.Set("continue", "https://chat.zalo.me/")

			tokenResp, err := u.Request(ctx, logoutTokenURL, &httpx.RequestOptions{
				Method: http.MethodPost,
				Body:   strings.NewReader(tokenForm.Encode()),
				Headers: http.Header{
					"Content-Type": []string{"application/x-www-form-urlencoded"},
				},
			})
			if err != nil {
				return nil, errs.WrapZCA("failed to request logout token", "api.Logout", err)
			}
			defer tokenResp.Body.Close()

			tokenBody, err := io.ReadAll(tokenResp.Body)
			if err != nil {
				return nil, errs.WrapZCA("failed to read logout token response", "api.Logout", err)
			}

			var tokenData struct {
				ErrorCode    int    `json:"error_code"`
				ErrorMessage string `json:"error_message"`
				Data         struct {
					Token string `json:"token"`
				} `json:"data"`
			}
			if err := json.Unmarshal(tokenBody, &tokenData); err != nil {
				return nil, errs.WrapZCA("failed to parse logout token json", "api.Logout", err)
			}
			if tokenData.ErrorCode != 0 || tokenData.Data.Token == "" {
				msg := tokenData.ErrorMessage
				if msg == "" {
					msg = fmt.Sprintf("error code %d", tokenData.ErrorCode)
				}
				return nil, errs.NewZCA("failed to get logout token: "+msg, "api.Logout")
			}

			// 2. Gửi logout token tới /account/logout
			logoutForm := url.Values{}
			logoutForm.Set("token", tokenData.Data.Token)

			logoutResp, err := u.Request(ctx, logoutURL, &httpx.RequestOptions{
				Method: http.MethodPost,
				Body:   strings.NewReader(logoutForm.Encode()),
				Headers: http.Header{
					"Content-Type": []string{"application/x-www-form-urlencoded"},
				},
			})
			if err != nil {
				return nil, errs.WrapZCA("failed to execute logout request", "api.Logout", err)
			}
			defer logoutResp.Body.Close()

			htmlBody, err := io.ReadAll(logoutResp.Body)
			if err != nil {
				return nil, errs.WrapZCA("failed to read logout html response", "api.Logout", err)
			}

			// 3. Phân tích regex tìm listSetSidUrls trong HTML
			matches := listSetSidRegex.FindSubmatch(htmlBody)
			if len(matches) < 2 {
				return nil, errs.NewZCA("failed to logout: setsid urls not found in response", "api.Logout")
			}

			var setSidURLs []string
			if err := json.Unmarshal(matches[1], &setSidURLs); err != nil {
				return nil, errs.WrapZCA("failed to parse setsid urls array", "api.Logout", err)
			}

			// 4. Hit từng setsid URL với Referer id.zalo.me để huỷ cookie
			var wg sync.WaitGroup
			for _, sidURL := range setSidURLs {
				if sidURL == "" {
					continue
				}
				wg.Add(1)
				go func(targetURL string) {
					defer wg.Done()
					resp, reqErr := u.Request(ctx, targetURL, &httpx.RequestOptions{
						Method: http.MethodGet,
						Headers: http.Header{
							"Referer": []string{"https://id.zalo.me/"},
						},
					})
					if reqErr == nil && resp != nil {
						_ = resp.Body.Close()
					}
				}(sidURL)
			}
			wg.Wait()

			// 5. Ngắt Listener
			if a.l != nil {
				a.l.Stop()
			}

			return &LogoutResponse{Status: true}, nil
		}, nil
	},
)
