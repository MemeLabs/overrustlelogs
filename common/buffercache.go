package common

import (
	"errors"
	"log"
	"math/rand"
	"runtime"
	"sort"
	"sync"
	"time"
)

// errors
var (
	ErrBufferTimeout = errors.New("ran out of time waiting for buffer")
)

// BufferCache ...
type BufferCache struct {
	sync.Mutex
	slices     [][]byte
	put        chan (bool)
	free       int64
	size       int64
	capacity   int64
	fillFactor float64
}

// NewBufferCache ...
func NewBufferCache(capacity int64, fillFactor float64) *BufferCache {
	return &BufferCache{
		slices:     [][]byte{},
		put:        make(chan bool),
		free:       capacity,
		size:       0,
		capacity:   capacity,
		fillFactor: fillFactor,
	}
}

// Get ...
func (c *BufferCache) Get(size int64) ([]byte, error) {
	eol := time.NewTimer(5 * time.Second)
	for {
		if c.free >= int64(size) {
			break
		}
		select {
		case <-c.put:
		case <-eol.C:
			return nil, ErrBufferTimeout
		}
	}
	// log.Printf("size %d, free %d", c.size, c.free)
	defer func() {
		if r := recover(); r != nil {
			log.Println("recovered", r)
		}
	}()
	c.Lock()
	defer c.Unlock()
	i := sort.Search(len(c.slices), func(i int) bool {
		return int64(cap(c.slices[i])) >= size
	})
	if i < len(c.slices) && int64(cap(c.slices[i])) > size {
		if float64(len(c.slices[i]))*c.fillFactor < float64(size) {
			s := c.slices[i]
			copy(c.slices[i:], c.slices[i+1:])
			c.slices = c.slices[:len(c.slices)-1]
			c.free -= int64(cap(s))
			return s[:size], nil
		}
	}
	gc := c.size+int64(size) < c.capacity
	for {
		if len(c.slices) == 0 || c.size+int64(size) < c.capacity {
			break
		}
		i := rand.Intn(len(c.slices))
		c.free += int64(len(c.slices[i]))
		c.size -= int64(len(c.slices[i]))
		copy(c.slices[i:], c.slices[i+1:])
		c.slices = c.slices[:len(c.slices)-1]
	}
	if gc {
		runtime.GC()
	}
	// log.Println("created buffer ", size)
	s := make([]byte, size)
	c.size += int64(cap(s))
	c.free -= int64(cap(s))
	return s, nil
}

// Put ...
func (c *BufferCache) Put(slice []byte) {
	// log.Printf("put %d", cap(slice))
	c.Lock()
	defer c.Unlock()
	defer func() {
		select {
		case c.put <- true:
		default:
		}
	}()
	if c.size+int64(cap(slice)) > c.capacity {
		c.size -= int64(cap(slice))
		return
	}
	c.free += int64(cap(slice))
	i := sort.Search(len(c.slices), func(i int) bool {
		return cap(c.slices[i]) >= cap(slice)
	})
	c.slices = append(c.slices, slice)
	if i < len(c.slices) {
		copy(c.slices[i+1:], c.slices[i:])
		c.slices[i] = slice
	} else if cap(c.slices[i]) >= cap(slice) {
		copy(c.slices[1:], c.slices)
		c.slices[0] = slice
	}
}
