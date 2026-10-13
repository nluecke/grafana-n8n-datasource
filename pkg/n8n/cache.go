package n8n

import (
	"sync"
	"time"
)

const maxCacheEntries = 2000

type cacheEntry struct {
	body    []byte
	expires time.Time
}

// responseCache stores raw response bodies per request URL for a fixed TTL.
type responseCache struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]cacheEntry
	now     func() time.Time
}

func newResponseCache(ttl time.Duration) *responseCache {
	return &responseCache{ttl: ttl, entries: map[string]cacheEntry{}, now: time.Now}
}

func (c *responseCache) get(key string) ([]byte, bool) {
	if c == nil || c.ttl <= 0 {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if c.now().After(e.expires) {
		delete(c.entries, key)
		return nil, false
	}
	return e.body, true
}

func (c *responseCache) set(key string, body []byte) {
	if c == nil || c.ttl <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if len(c.entries) >= maxCacheEntries {
		for k, e := range c.entries {
			if now.After(e.expires) {
				delete(c.entries, k)
			}
		}
		if len(c.entries) >= maxCacheEntries {
			c.entries = map[string]cacheEntry{}
		}
	}
	c.entries[key] = cacheEntry{body: body, expires: now.Add(c.ttl)}
}
