package common

import "time"

// immutable config
const (
	SocketHandshakeTimeout = 10 * time.Second
	SocketReconnectDelay   = 20 * time.Second
	SocketWriteDebounce    = 500 * time.Millisecond
	SocketReadTimeout      = 20 * time.Second
	MessageBufferSize      = 100
)
