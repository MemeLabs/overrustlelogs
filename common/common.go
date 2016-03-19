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
	MessageBufferSize      = 100
)

// ErrNotConnected ...
var ErrNotConnected = errors.New("not connected")
