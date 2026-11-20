package cache

import (
	"sync"
	"testing"
	"time"
)

func TestMemoryCacheSetGet(t *testing.T) {
	c := NewMemoryCache(1 * time.Minute)
	c.Set("k1", "v1")
	got, ok := c.Get("k1")
	if !ok || got != "v1" {
		t.Errorf("Get(k1) = (%v, %v), want (v1, true)", got, ok)
	}
}

func TestMemoryCacheMiss(t *testing.T) {
	c := NewMemoryCache(1 * time.Minute)
	if _, ok := c.Get("missing"); ok {
		t.Error("不存在的键不应命中")
	}
}

func TestMemoryCacheExpiration(t *testing.T) {
	c := NewMemoryCache(30 * time.Millisecond)
	c.Set("k", "v")
	time.Sleep(60 * time.Millisecond)
	if _, ok := c.Get("k"); ok {
		t.Error("过期键不应命中")
	}
}

func TestMemoryCacheDeleteAndClear(t *testing.T) {
	c := NewMemoryCache(1 * time.Minute)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Delete("a")
	if _, ok := c.Get("a"); ok {
		t.Error("Delete 后不应命中")
	}
	if _, ok := c.Get("b"); !ok {
		t.Error("未删除的键应保留")
	}
	c.Clear()
	if _, ok := c.Get("b"); ok {
		t.Error("Clear 后不应有剩余键")
	}
}

func TestMemoryCacheConcurrentAccess(t *testing.T) {
	c := NewMemoryCache(1 * time.Minute)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := "key"
			c.Set(key, i)
			c.Get(key)
			c.Delete(key)
		}(i)
	}
	wg.Wait()
}

func TestDefaultCacheSingleton(t *testing.T) {
	c1 := GetDefaultCache()
	c2 := GetDefaultCache()
	if c1 != c2 {
		t.Error("GetDefaultCache 应返回同一单例")
	}
}

func TestUserCache(t *testing.T) {
	uc := NewUserCache(1 * time.Minute)
	if _, ok := uc.GetUser("42"); ok {
		t.Error("未设置的用户不应命中")
	}
	user := map[string]interface{}{"id": 42, "email": "a@b.c"}
	uc.SetUser("42", user)
	got, ok := uc.GetUser("42")
	if !ok || got["email"] != "a@b.c" {
		t.Errorf("GetUser(42) = (%v, %v)", got, ok)
	}
	uc.InvalidateUser("42")
	if _, ok := uc.GetUser("42"); ok {
		t.Error("InvalidateUser 后不应命中")
	}
}
