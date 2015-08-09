package logger

import (
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hashicorp/golang-lru"
	"github.com/slugalisk/overrustlelogs/common"
)

var empty struct{}

// ChatLog handles single log file
type ChatLog struct {
	sync.Mutex
	f        *os.File
	nicks    common.NickList
	modified time.Time
}

// NewChatLog instantiates chat logs...
func NewChatLog(path string) (*ChatLog, error) {
	dir := filepath.Dir(path)
	_, err := os.Stat(dir)
	if err != nil {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return nil, err
		}
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	nicks := common.NickList{}
	nicks.ReadFrom(nickPath(path))

	return &ChatLog{
		f:        f,
		nicks:    nicks,
		modified: time.Now(),
	}, nil
}

// WriteNicks persist nick list
func (l *ChatLog) WriteNicks() {
	l.Lock()
	if err := l.nicks.WriteTo(nickPath(l.f.Name())); err != nil {
		log.Printf("error writing nicks for %s %s", l.f.Name(), err)
	}
	l.Unlock()
}

// Close release file handle
func (l *ChatLog) Close() {
	l.WriteNicks()
	l.Lock()
	l.f.Close()
	l.Unlock()
}

func (l *ChatLog) Write(timestamp time.Time, nick string, message string) {
	l.Lock()
	l.nicks.Add(nick)
	l.f.WriteString(timestamp.Format("[2006-01-02 15:04:05 MST] ") + nick + ": " + message + "\n")
	l.modified = time.Now()
	l.Unlock()
}

func nickPath(path string) string {
	ext := filepath.Ext(path)
	return path[:len(path)-len(ext)] + ".nicks"
}

// ChatLogs chat log collection
type ChatLogs struct {
	logs *lru.Cache
}

// NewChatLogs instantiates chat log collection
func NewChatLogs() *ChatLogs {
	l := &ChatLogs{}
	cache, err := lru.NewWithEvict(common.GetConfig().MaxOpenLogs/2, l.HandleEvict)
	if err != nil {
		log.Fatalf("error creating log cache %s", err)
	}
	l.logs = cache

	go func() {
		for {
			time.Sleep(1 * time.Minute)
			for _, k := range cache.Keys() {
				if v, ok := cache.Peek(k); ok {
					v.(*ChatLog).WriteNicks()
				}
			}
		}
	}()

	return l
}

// HandleEvict close evicted logs
func (l *ChatLogs) HandleEvict(key interface{}, chatLog interface{}) {
	chatLog.(*ChatLog).Close()
}

// Get returns chat log for the supplied path
func (l *ChatLogs) Get(path string) (*ChatLog, error) {
	if chatLog, ok := l.logs.Get(path); ok {
		return chatLog.(*ChatLog), nil
	}

	chatLog, err := NewChatLog(path)
	if err != nil {
		return nil, err
	}

	l.logs.Add(path, chatLog)

	return chatLog, nil
}
