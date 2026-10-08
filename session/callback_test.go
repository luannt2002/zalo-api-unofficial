package session

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/luannt2002/zalo-api-unofficial/model"
)

func waitSignal(t *testing.T, ch <-chan string, what string) string {
	t.Helper()
	select {
	case k := <-ch:
		return k
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
		return ""
	}
}

func TestCallbacksMapTTLEvictionFiresOnExpire(t *testing.T) {
	t.Parallel()

	c := NewCallbacksMap()
	expired := make(chan string, 4)
	var fired atomic.Int32

	c.Set("file-1", func(model.UploadAttachment) {}, func(key string) {
		fired.Add(1)
		expired <- key
	}, 20*time.Millisecond)

	if got := waitSignal(t, expired, "TTL expiry"); got != "file-1" {
		t.Fatalf("onExpire key = %q, want %q", got, "file-1")
	}
	if _, ok := c.Get("file-1"); ok {
		t.Fatal("entry should be evicted after TTL")
	}
	if c.Len() != 0 {
		t.Fatalf("Len() = %d, want 0", c.Len())
	}

	// Give a stray duplicate notification time to show up.
	time.Sleep(50 * time.Millisecond)
	if n := fired.Load(); n != 1 {
		t.Fatalf("onExpire fired %d times, want exactly 1", n)
	}
}

func TestCallbacksMapDeleteAndConsumeSuppressOnExpire(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		consume bool // true: Get+invoke before Delete (the router flow)
	}{
		{name: "explicit delete before TTL", consume: false},
		{name: "consume via Get then Delete", consume: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := NewCallbacksMap()
			var expireFired, cbFired atomic.Int32

			c.Set("k", func(model.UploadAttachment) { cbFired.Add(1) }, func(string) {
				expireFired.Add(1)
			}, 30*time.Millisecond)

			if tt.consume {
				cb, ok := c.Get("k")
				if !ok {
					t.Fatal("Get returned no callback")
				}
				cb(model.UploadAttachment{})
			}
			c.Delete("k")

			time.Sleep(100 * time.Millisecond) // well past TTL

			if n := expireFired.Load(); n != 0 {
				t.Fatalf("onExpire fired %d times after Delete, want 0", n)
			}
			if tt.consume && cbFired.Load() != 1 {
				t.Fatalf("callback fired %d times, want 1", cbFired.Load())
			}
			if c.Len() != 0 {
				t.Fatalf("Len() = %d, want 0", c.Len())
			}
		})
	}
}

func TestCallbacksMapReRegisterReplacesEntryAndTimer(t *testing.T) {
	t.Parallel()

	c := NewCallbacksMap()
	var firstExpired atomic.Int32
	expired := make(chan string, 4)

	c.Set("k", func(model.UploadAttachment) {}, func(string) {
		firstExpired.Add(1)
	}, 25*time.Millisecond)

	// Replace before the first TTL fires; only the second onExpire may run.
	c.Set("k", func(model.UploadAttachment) {}, func(key string) {
		expired <- key
	}, 50*time.Millisecond)

	waitSignal(t, expired, "second registration expiry")
	time.Sleep(50 * time.Millisecond)

	if n := firstExpired.Load(); n != 0 {
		t.Fatalf("replaced entry's onExpire fired %d times, want 0", n)
	}
	if c.Len() != 0 {
		t.Fatalf("Len() = %d, want 0", c.Len())
	}
}

func TestCallbacksMapCloseNotifiesAllPending(t *testing.T) {
	t.Parallel()

	c := NewCallbacksMap()
	keys := []string{"a", "b", "c"}

	var mu sync.Mutex
	notified := make(map[string]int, len(keys))

	for _, k := range keys {
		c.Set(k, func(model.UploadAttachment) {}, func(key string) {
			mu.Lock()
			notified[key]++
			mu.Unlock()
		}, time.Minute)
	}

	c.Close()

	mu.Lock()
	defer mu.Unlock()
	for _, k := range keys {
		if notified[k] != 1 {
			t.Fatalf("onExpire for %q fired %d times, want 1", k, notified[k])
		}
	}
	if c.Len() != 0 {
		t.Fatalf("Len() = %d after Close, want 0", c.Len())
	}
}

// TestCallbacksMapConcurrentAccess is a race-detector smoke test for
// interleaved Set/Get/Delete with very short TTLs.
func TestCallbacksMapConcurrentAccess(t *testing.T) {
	t.Parallel()

	c := NewCallbacksMap()
	var wg sync.WaitGroup

	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				c.Set("shared", func(model.UploadAttachment) {}, func(string) {}, time.Millisecond)
				if cb, ok := c.Get("shared"); ok && cb != nil {
					cb(model.UploadAttachment{})
				}
				c.Delete("shared")
			}
		}()
	}
	wg.Wait()

	c.Close()
	if c.Len() != 0 {
		t.Fatalf("Len() = %d after Close, want 0", c.Len())
	}
}
