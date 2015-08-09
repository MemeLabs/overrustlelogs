package logger

import (
	"log"
	"time"
)

// immutable config
const (
	SocketHandshakeTimeout = 10 * time.Second
	SocketReconnectDelay   = 20 * time.Second
	SocketWriteDebounce    = 500 * time.Millisecond
	SocketReadTimeout      = 20 * time.Second
	MessageBufferSize      = 100
)

// Start logger
func Start() {
	logs := NewChatLogs()

	dc := NewDestinyChat()
	dl := NewDestinyLogger(logs)
	go dl.Log(dc.Messages())
	go dc.Run()

	tc := NewTwitchChat(func(ch string, m chan *Message) {
		log.Printf("started logging %s", ch)
		NewTwitchLogger(logs, ch).Log(m)
		log.Printf("stopped logging %s", ch)
	})
	go tc.Run()

	select {}
}
