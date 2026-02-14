package cache

import (
	"sync"
	"time"
)

type CacheItem struct {
	Value      interface{}
	Expiration time.Time
}

type MemoryCache struct {
	mu    sync.RWMutex
	items map[string]CacheItem
	ttl   time.Duration
}

var (
	defaultCache *MemoryCache
	once         sync.Once
)

func GetDefaultCache() *MemoryCache {
	once.Do(func() {
		defaultCache = NewMemoryCache(5 * time.Minute)
	})
	return defaultCache
}

func NewMemoryCache(ttl time.Duration) *MemoryCache {
	c := &MemoryCache{
		items: make(map[string]CacheItem),
		ttl:   ttl,
	}
	go c.cleanup()
	return c
}

func (c *MemoryCache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = CacheItem{
		Value:      value,
		Expiration: time.Now().Add(c.ttl),
	}
}

func (c *MemoryCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	item, found := c.items[key]
	if !found {
		return nil, false
	}
	if time.Now().After(item.Expiration) {
		return nil, false
	}
	return item.Value, true
}

func (c *MemoryCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

func (c *MemoryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]CacheItem)
}

func (c *MemoryCache) cleanup() {
	ticker := time.NewTicker(c.ttl)
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, item := range c.items {
			if now.After(item.Expiration) {
				delete(c.items, key)
			}
		}
		c.mu.Unlock()
	}
}

type UserCache struct {
	cache *MemoryCache
}

func NewUserCache(ttl time.Duration) *UserCache {
	return &UserCache{
		cache: NewMemoryCache(ttl),
	}
}

func (uc *UserCache) GetUser(userID string) (map[string]interface{}, bool) {
	val, found := uc.cache.Get("user:" + userID)
	if !found || val == nil {
		return nil, false
	}
	user, ok := val.(map[string]interface{})
	return user, ok
}

func (uc *UserCache) SetUser(userID string, user map[string]interface{}) {
	uc.cache.Set("user:"+userID, user)
}

func (uc *UserCache) InvalidateUser(userID string) {
	uc.cache.Delete("user:" + userID)
}
