package common

import (
	"fmt"
	"regexp"
	"time"
)

// const ...
const (
	HandshakeTimeout     = 10 * time.Second
	MaxChannelsPerChat   = 50
	MessageBufferSize    = 1000
	SocketReadTimeout    = 10 * time.Minute
	SocketReconnectDelay = 20 * time.Second
	SocketWriteDebounce  = 500 * time.Millisecond
	SocketWriteTimeout   = 5 * time.Second
)

var messageNickPathUnsafe = regexp.MustCompile("[^a-zA-Z0-9_-]")

// Message data
type Message struct {
	Type    string
	Channel string
	Nick    string
	Data    string
	Time    time.Time
}

func (m *Message) String() string {
	return fmt.Sprintf("#%s : < %s > : %s", m.Channel, m.Nick, m.Data)
}

// NickPath return sanitized nick for use with fs
func (m *Message) NickPath() string {
	return messageNickPathUnsafe.ReplaceAllString(m.Nick, "")
}
