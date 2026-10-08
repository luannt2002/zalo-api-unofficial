package session

import (
	"sync"
	"time"

	"github.com/luannt2002/zalo-api-unofficial/config"
	"github.com/luannt2002/zalo-api-unofficial/model"
)

type UploadCallback = func(data model.UploadAttachment)

// UploadExpiredCallback is invoked when an entry is evicted by TTL (or when the
// map is closed) before the callback was consumed. It gives waiters a cancel
// signal so they never block forever on an event that will not arrive.
type UploadExpiredCallback = func(key string)

type callbackEntry struct {
	cb       UploadCallback
	onExpire UploadExpiredCallback
}

type CallbacksMap struct {
	mu     sync.Mutex
	items  map[string]callbackEntry
	timers map[string]*time.Timer
}

func NewCallbacksMap() *CallbacksMap {
	return &CallbacksMap{
		items:  make(map[string]callbackEntry),
		timers: make(map[string]*time.Timer),
	}
}

// Set registers value under key with the given TTL (<=0 means
// config.DefaultUploadCallbackTTL). If the entry is still registered when the
// TTL fires, it is evicted and onExpire (if non-nil) is called exactly once,
// outside the map lock. An explicit Delete (the consume path) prevents the
// expiry notification.
func (c *CallbacksMap) Set(key string, value UploadCallback, onExpire UploadExpiredCallback, ttl time.Duration) *CallbacksMap {
	if ttl <= 0 {
		ttl = config.DefaultUploadCallbackTTL
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if t, ok := c.timers[key]; ok {
		t.Stop()
	}
	c.items[key] = callbackEntry{cb: value, onExpire: onExpire}

	var timer *time.Timer
	timer = time.AfterFunc(ttl, func() {
		c.mu.Lock()
		// A stopped timer that already fired can still reach this point after
		// the key was deleted or re-registered; only the current timer for the
		// key may evict the entry.
		if cur, ok := c.timers[key]; !ok || cur != timer {
			c.mu.Unlock()
			return
		}
		entry := c.items[key]
		delete(c.items, key)
		delete(c.timers, key)
		c.mu.Unlock()

		if entry.onExpire != nil {
			entry.onExpire(key)
		}
	})
	c.timers[key] = timer

	return c
}

func (c *CallbacksMap) Get(key string) (UploadCallback, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.items[key]
	if !ok {
		return nil, false
	}
	return entry.cb, true
}

func (c *CallbacksMap) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
	if t, ok := c.timers[key]; ok {
		t.Stop()
		delete(c.timers, key)
	}
}

func (c *CallbacksMap) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.items)
}

// Close evicts every pending entry and notifies their onExpire handlers so
// in-flight waiters are released instead of leaking until their own timeout.
func (c *CallbacksMap) Close() {
	c.mu.Lock()
	for k, t := range c.timers {
		t.Stop()
		delete(c.timers, k)
	}
	expired := make([]callbackEntry, 0, len(c.items))
	keys := make([]string, 0, len(c.items))
	for k, entry := range c.items {
		if entry.onExpire != nil {
			expired = append(expired, entry)
			keys = append(keys, k)
		}
	}
	clear(c.items)
	c.mu.Unlock()

	for i, entry := range expired {
		entry.onExpire(keys[i])
	}
}
