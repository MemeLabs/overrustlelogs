package main

import (
	"log"
	"sync/atomic"

	"github.com/hashicorp/golang-lru"
	"github.com/slugalisk/overrustlelogs/common"
)

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
	atomic.AddInt64(l.size, -item.(*logCacheItem).size)
	bufferCache.Put(item.(*logCacheItem).data[0])
}

func (l *logCache) add(key string, data [][]byte, size int64) {
	if size > common.GetConfig().Server.MaxLogCacheSize {
		bufferCache.Put(data[0])
		return
	}
	for {
		if atomic.LoadInt64(l.size)+size < common.GetConfig().Server.MaxLogCacheSize {
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
