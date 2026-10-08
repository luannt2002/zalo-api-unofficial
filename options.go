package zcago

import (
	"context"
	"net/http"

	"github.com/luannt2002/zalo-api-unofficial/session"
	"github.com/luannt2002/zalo-api-unofficial/session/auth"
)

type (
	LoginQREvent    auth.LoginQREvent
	LoginQRCallback auth.LoginQRCallback
)

type LoginQROption struct {
	UserAgent string
	Language  string
	QRPath    string
}

func login(ctx context.Context, sc session.MutableContext, encryptParams bool) (*session.LoginInfo, error) {
	return auth.Login(ctx, sc, encryptParams)
}

func getServerInfo(ctx context.Context, sc session.MutableContext, enableEncryptParam bool) (*session.ServerInfo, error) {
	return auth.GetServerInfo(ctx, sc, enableEncryptParam)
}

func loginQR(ctx context.Context, sc session.MutableContext, qrPath string, cb LoginQRCallback) (*auth.LoginQRResult, error) {
	return auth.LoginQR(ctx, sc, qrPath, auth.LoginQRCallback(cb))
}

func WithSelfListen(v bool) session.Option  { return session.WithSelfListen(v) }
func WithCheckUpdate(v bool) session.Option { return session.WithCheckUpdate(v) }
func WithLogging(v bool) session.Option     { return session.WithLogging(v) }

// WithLogLevel sets the logging verbosity for a session.
//
// Accepted values:
//
//	0 — Verbose
//	1 — Debug
//	2 — Info
//	3 — Warn
//	4 — Error
//	5 — Success
func WithLogLevel(level uint8) session.Option      { return session.WithLogLevel(level) }
func WithHTTPClient(c *http.Client) session.Option { return session.WithHTTPClient(c) }

// WithProxy routes all session traffic (REST + WebSocket) through the given
// HTTP/HTTPS proxy URL, e.g. "http://user:pass@host:port".
//
// Used to spread bot logins across different egress IPs to avoid IP bans.
// An empty string is a no-op (direct connection). SOCKS5 proxies are also
// supported via socks5:// URLs (handled by net/http.Transport).
func WithProxy(proxyURL string) session.Option { return session.WithProxy(proxyURL) }
func WithAPIType(t uint) session.Option            { return session.WithAPIType(t) }
func WithAPIVersion(v uint) session.Option         { return session.WithAPIVersion(v) }

func WithImageMetadataGetter(f session.ImageMetadataGetter) session.Option {
	return session.WithImageMetadataGetter(f)
}
