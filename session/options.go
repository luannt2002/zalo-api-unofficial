package session

import (
	"net/http"
	"net/url"

	"github.com/luannt2002/zalo-api-unofficial/config"
	"github.com/luannt2002/zalo-api-unofficial/model"
)

type Option func(*options)

type OptionsSnapshot struct {
	SelfListen          bool
	CheckUpdate         bool
	Logging             bool
	LogLevel            uint8
	APIType             uint
	APIVersion          uint
	Client              *http.Client
	ProxyURL            *url.URL
	ImageMetadataGetter ImageMetadataGetter
}

type options struct {
	selfListen  bool
	checkUpdate bool
	logging     bool
	logLevel    uint8
	apiType     uint
	apiVersion  uint

	client *http.Client

	// proxyURL, when set, routes both REST and WebSocket traffic through the
	// given HTTP/HTTPS proxy. Applied lazily in defaultOptions/newContextImpl so
	// it does not mutate http.DefaultClient or a user-supplied client's transport
	// in place.
	proxyURL *url.URL

	imageMetadataGetter ImageMetadataGetter
}

func WithSelfListen(v bool) Option         { return func(o *options) { o.selfListen = v } }
func WithCheckUpdate(v bool) Option        { return func(o *options) { o.checkUpdate = v } }
func WithLogging(v bool) Option            { return func(o *options) { o.logging = v } }
func WithLogLevel(level uint8) Option      { return func(o *options) { o.logLevel = level } }
func WithHTTPClient(c *http.Client) Option { return func(o *options) { o.client = c } }

// WithProxy routes all session traffic (REST + WebSocket) through the given
// HTTP/HTTPS proxy URL, e.g. "http://user:pass@host:port" or "https://host:port".
//
// An empty proxyURL is a no-op (direct connection). An unparsable URL is ignored
// so a single bad value never breaks an otherwise valid session; callers that
// need strict validation should parse the URL themselves first.
//
// SOCKS5 proxies are not handled here; net/http only understands http/https
// (and socks5 via golang.org/x/net/proxy, which is intentionally left for later).
func WithProxy(proxyURL string) Option {
	return func(o *options) {
		if proxyURL == "" {
			o.proxyURL = nil
			return
		}
		u, err := url.Parse(proxyURL)
		if err != nil || u.Host == "" {
			return
		}
		o.proxyURL = u
	}
}

func WithAPIType(t uint) Option {
	return func(o *options) {
		if t != 0 {
			o.apiType = t
		}
	}
}

func WithAPIVersion(v uint) Option {
	return func(o *options) {
		if v != 0 {
			o.apiVersion = v
		}
	}
}

func WithImageMetadataGetter(f ImageMetadataGetter) Option {
	return func(o *options) { o.imageMetadataGetter = f }
}

func defaultOptions() options {
	return options{
		selfListen:  false,
		checkUpdate: true,
		logging:     true,
		logLevel:    1, // Debug level by default
		apiType:     config.DefaultAPIType,
		apiVersion:  config.DefaultAPIVersion,
		client:      http.DefaultClient,
	}
}

type ImageMetadataGetter func(filePath string) (model.AttachmentMetadata, error)
