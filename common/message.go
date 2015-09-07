package common

import (
	"regexp"
	"time"
)

var messageNickPathUnsafe = regexp.MustCompile("[^a-zA-Z0-9_-]")

// Message data
type Message struct {
	Command string
	Nick    string
	Data    string
	Time    time.Time
}

// NickPath return sanitized nick for use with fs
func (m *Message) NickPath() string {
	return messageNickPathUnsafe.ReplaceAllString(m.Nick, "")
}
