package common

import (
	"errors"
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

// errors
var ErrNotConnected = errors.New("not connected")
