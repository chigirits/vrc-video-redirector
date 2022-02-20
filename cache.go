package main

import (
	"strconv"
	"sync"
	"time"

	"github.com/goware/urlx"
)

type CacheEntry struct {
	Expire time.Time
	Format *VideoFormat
	Info   *VideoInfo
}

type Cache struct {
	Mutex   sync.Mutex
	entries map[string]*CacheEntry
}

func NewCache() *Cache {
	return &Cache{
		entries: make(map[string]*CacheEntry),
	}
}

func (c *Cache) Store(videoFormat *VideoFormat, videoInfo *VideoInfo, resolvedURL string) bool {
	r, err := urlx.Parse(videoFormat.URL)
	if err != nil {
		return false
	}
	q := r.Query()
	if len(q["expire"]) == 0 {
		return false
	}
	expireUnix, err := strconv.Atoi(q["expire"][0])
	if err != nil {
		return false
	}
	expire := time.Unix(int64(expireUnix), 0)
	cache.entries[resolvedURL] = &CacheEntry{
		Expire: expire,
		Format: videoFormat,
		Info:   videoInfo,
	}
	return true
}

func (c *Cache) Load(url string) (*CacheEntry, bool) {
	entry, ok := c.entries[url]
	if !ok {
		return nil, false
	}
	if time.Now().Before(entry.Expire) {
		return entry, true
	}
	delete(cache.entries, url)
	return nil, false
}
