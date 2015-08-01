package main

import (
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hashicorp/golang-lru"
)

// ChatLog handles single log file
type ChatLog struct {
	sync.Mutex
	f *os.File
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

	return &ChatLog{f: f}, nil
}

// Close release file handle
func (l *ChatLog) Close() {
	l.Lock()
	l.f.Close()
	l.Unlock()
}

func (l *ChatLog) Write(timestamp time.Time, line string) {
	l.Lock()
	l.f.WriteString(timestamp.Format("[2006-01-02 15:04:05 MST] ") + line + "\n")
	l.Unlock()
}

// ChatLogs chat log collection
type ChatLogs struct {
	logs *lru.Cache
}

// NewChatLogs instantiates chat log collection
func NewChatLogs() *ChatLogs {
	l := &ChatLogs{}
	cache, err := lru.NewWithEvict(config.MaxOpenLogs, l.HandleEvict)
	if err != nil {
		log.Fatalf("error creating log cache %s", err)
	}
	l.logs = cache

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
