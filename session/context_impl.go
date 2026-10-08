package session

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"github.com/luannt2002/zalo-api-unofficial/config"
	"github.com/luannt2002/zalo-api-unofficial/errs"
	"github.com/luannt2002/zalo-api-unofficial/model"
)

type contextImpl struct {
	mu sync.RWMutex

	uid       string
	imei      string
	userAgent string
	language  string

	apiType    uint
	apiVersion uint

	opts OptionsSnapshot

	secretKey SecretKey
	loginInfo *LoginInfo
	settings  *Settings
	extraVer  *ExtraVer
	jar       http.CookieJar

	uploadCallbacks *CallbacksMap
}

func newContextImpl(optFns ...Option) *contextImpl {
	cfg := defaultOptions()
	for _, fn := range optFns {
		if fn != nil {
			fn(&cfg)
		}
	}

	// Always work on a shallow clone of the configured client. We never mutate
	// http.DefaultClient (or a caller-supplied client) in place, so concurrent
	// sessions can't leak cookies, redirect policy or proxy settings onto each
	// other.
	base := cfg.client
	if base == nil {
		base = http.DefaultClient
	}
	cloned := *base
	client := &cloned

	// Each session gets its own cookie jar so cookies from different Zalo
	// accounts never mix. The only exception: the caller explicitly provided a
	// jar on a non-default client they own.
	var jar http.CookieJar
	if cfg.client != nil && cfg.client != http.DefaultClient && cfg.client.Jar != nil {
		jar = cfg.client.Jar
	} else {
		jar, _ = cookiejar.New(nil)
	}
	client.Jar = jar

	// Redirects are followed manually in internal/httpx (requestWithRedirect),
	// so the client itself must not auto-follow. Set once on the session-owned
	// clone instead of mutating a possibly shared client per request.
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	// Note: intentionally no client.Timeout here — this same client serves
	// long-running chunked uploads and the websocket handshake, where a global
	// timeout could cut off slow-but-healthy transfers. Per-request deadlines
	// come from the caller's context instead.
	if cfg.proxyURL != nil {
		client = applyProxy(client, cfg.proxyURL)
	}

	return &contextImpl{
		apiType:    cfg.apiType,
		apiVersion: cfg.apiVersion,
		opts: OptionsSnapshot{
			SelfListen:          cfg.selfListen,
			CheckUpdate:         cfg.checkUpdate,
			Logging:             cfg.logging,
			LogLevel:            cfg.logLevel,
			APIType:             cfg.apiType,
			APIVersion:          cfg.apiVersion,
			Client:              client,
			ProxyURL:            cfg.proxyURL,
			ImageMetadataGetter: cfg.imageMetadataGetter,
		},
		jar:             jar,
		uploadCallbacks: NewCallbacksMap(),
		language:        config.DefaultLanguage,
	}
}

// applyProxy returns a copy of client whose transport routes requests through
// proxyURL. The returned client and its *http.Transport are clones, so the
// global http.DefaultTransport / http.DefaultClient are never mutated.
//
// Only *http.Transport is proxy-aware. If the supplied client carries a custom
// (non-*http.Transport) RoundTripper, we cannot inject a proxy into it without
// breaking its behaviour, so we leave it untouched and let the caller's
// transport decide. Both REST and WebSocket reuse this same client (the
// websocket dialer is given sc.Client()), so a single call proxies everything.
func applyProxy(client *http.Client, proxyURL *url.URL) *http.Client {
	if client == nil {
		client = http.DefaultClient
	}

	cloned := *client // shallow copy: Jar/Timeout/etc. preserved

	switch tr := client.Transport.(type) {
	case *http.Transport:
		t := tr.Clone()
		t.Proxy = http.ProxyURL(proxyURL)
		cloned.Transport = t
	case nil:
		t := http.DefaultTransport.(*http.Transport).Clone()
		t.Proxy = http.ProxyURL(proxyURL)
		cloned.Transport = t
	default:
		// Custom RoundTripper: respect it, proxy injection not possible here.
	}

	return &cloned
}

// ----------------------------------------
// State Management
// ----------------------------------------

type Seal struct {
	UID       string
	IMEI      string
	UserAgent string
	Language  string

	SecretKey SecretKey
	LoginInfo *LoginInfo
	Settings  *Settings
	ExtraVer  *ExtraVer
	Jar       http.CookieJar
}

func (c *contextImpl) SealLogin(s Seal) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if s.Jar != nil {
		c.jar = s.Jar
	}
	c.uid = s.UID
	c.imei = s.IMEI
	if s.UserAgent != "" {
		c.userAgent = s.UserAgent
	}
	if s.Language != "" {
		c.language = s.Language
	}

	c.secretKey = s.SecretKey
	c.loginInfo = s.LoginInfo
	c.settings = s.Settings
	c.extraVer = s.ExtraVer
}

func (c *contextImpl) Client() *http.Client { return c.opts.Client }
func (c *contextImpl) Proxy() func(*http.Request) (*url.URL, error) {
	client := c.opts.Client

	transport, ok := client.Transport.(*http.Transport)
	if !ok || transport == nil {
		transport = http.DefaultTransport.(*http.Transport)
	}

	return transport.Proxy
}

func (c *contextImpl) SetIMEI(imei string)     { c.mu.Lock(); c.imei = imei; c.mu.Unlock() }
func (c *contextImpl) SetUserAgent(ua string)  { c.mu.Lock(); c.userAgent = ua; c.mu.Unlock() }
func (c *contextImpl) SetLanguage(lang string) { c.mu.Lock(); c.language = lang; c.mu.Unlock() }

func (c *contextImpl) SetCookieJar(j http.CookieJar) {
	c.mu.Lock()
	c.jar = j
	if c.opts.Client != nil {
		c.opts.Client.Jar = j
	}
	c.mu.Unlock()
}

func (c *contextImpl) CookieJar() http.CookieJar {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.jar
}

func (c *contextImpl) AsReadOnly() Context { return c }

// ----------------------------------------
// Context API
// ----------------------------------------

func (c *contextImpl) UID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.uid
}

func (c *contextImpl) IMEI() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.imei
}

func (c *contextImpl) UserAgent() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.userAgent
}

func (c *contextImpl) Language() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.language
}

func (c *contextImpl) APIType() uint    { return c.apiType }
func (c *contextImpl) APIVersion() uint { return c.apiVersion }

func (c *contextImpl) Options() OptionsSnapshot { return c.opts }
func (c *contextImpl) IsLogging() bool          { return c.opts.Logging }
func (c *contextImpl) LogLevel() uint8          { return c.opts.LogLevel }
func (c *contextImpl) CheckUpdate() bool        { return c.opts.CheckUpdate }
func (c *contextImpl) GetImageMetadata(path string) (model.AttachmentMetadata, string, error) {
	getter := c.opts.ImageMetadataGetter
	if getter == nil {
		return model.AttachmentMetadata{}, "", errs.ErrMissingImageMetadataGetter
	}

	meta, err := getter(path)
	if err != nil {
		return model.AttachmentMetadata{}, "", errs.WrapZCA("failed to get image metadata", "context.GetImageMetadata", err)
	}

	fileName := filepath.Base(path)
	return meta, fileName, nil
}

func (c *contextImpl) LoginInfo() *LoginInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.loginInfo
}

func (c *contextImpl) Settings() *Settings {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.settings
}

func (c *contextImpl) ExtraVer() *ExtraVer {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.extraVer
}

func (c *contextImpl) SecretKey() SecretKey {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.secretKey
}

func (c *contextImpl) UploadCallback() *CallbacksMap {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.uploadCallbacks
}

func (c *contextImpl) ZPWServiceMap() *ZpwServiceMap {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.loginInfo == nil {
		return nil
	}
	return &c.loginInfo.ZpwServiceMapV3
}

func (c *contextImpl) WSPingInterval() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.settings == nil ||
		c.settings.Features.Socket.PingInterval <= 0 {
		return 0
	}
	return time.Duration(c.settings.Features.Socket.PingInterval) * time.Millisecond
}

func (c *contextImpl) GetZpwService(service string) []string {
	sm := c.ZPWServiceMap()
	if sm == nil {
		return nil
	}

	val := reflect.ValueOf(sm).Elem()
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag == service {
			return val.Field(i).Interface().([]string)
		}
	}
	return nil
}

func (c *contextImpl) ZPWWebsocket() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.loginInfo == nil || len(c.loginInfo.ZpwWebsocket) == 0 {
		return nil
	}
	return c.loginInfo.ZpwWebsocket
}
