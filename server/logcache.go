package main

import (
	"log"
	"sync/atomic"

	"github.com/hashicorp/golang-lru"
)

const maxLogCacheSize = int64(500000000)

type logCache struct {
	c    *lru.Cache
	size *int64
}

type logCacheItem struct {
	size int64
	data [][]byte
}

func newLogCache() *logCache {
	var s int64
	l := &logCache{size: &s}

	c, err := lru.NewWithEvict(1000, l.handleEvict)
	if err != nil {
		log.Fatalf("error creating log cache %s", err)
	}
	l.c = c

	return l
}

func (l *logCache) handleEvict(key interface{}, item interface{}) {
	atomic.AddInt64(l.size, -item.(logCacheItem).size)
}

func (l *logCache) add(key string, data [][]byte, size int64) {
	if size > maxLogCacheSize {
		return
	}
	for {
		if atomic.LoadInt64(l.size)+size < maxLogCacheSize {
			break
		}
		l.c.RemoveOldest()
	}
	l.c.Add(key, &logCacheItem{size, data})
	atomic.AddInt64(l.size, size)
}

func (l *logCache) get(key string) ([][]byte, bool) {
	item, ok := l.c.Get(key)
	if !ok {
		return nil, false
	}
	return item.(*logCacheItem).data, true
}
