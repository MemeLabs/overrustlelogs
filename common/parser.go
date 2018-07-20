package common

import (
	"bytes"
	"fmt"
	"regexp"
	"time"
)

// Log timestamp format
var (
	MessageTimeLayout       = "[2006-01-02 15:04:05 MST] "
	MessageTimeLayoutLength = len(MessageTimeLayout)
	MessageDateLayout       = "2006-01-02"
)

// ParseMessageLine parse log line into message struct
func ParseMessageLine(b []byte) (*Message, error) {
	if len(b) < MessageTimeLayoutLength {
		return nil, fmt.Errorf("supplied line is too short to be parsed as message: %s", b)
	}

	ts, err := time.Parse(MessageTimeLayout, string(b[:MessageTimeLayoutLength]))
	if err != nil {
		return nil, fmt.Errorf("malformed date in message: %v", err)
	}

	b = b[MessageTimeLayoutLength:]
	nickLength := bytes.IndexRune(b, ':')
	if nickLength == len(b) || nickLength > len(b) {
		return nil, fmt.Errorf("malformed nick in message: %s", b)
	}

	return &Message{
		Nick: string(b[:nickLength]),
		Data: string(b[nickLength+2:]),
		Time: ts,
	}, nil
}

var channelPathPattern = regexp.MustCompile("/([a-zA-Z0-9_]+) chatlog/")

// ExtractChannelFromPath ...
func ExtractChannelFromPath(p string) (string, error) {
	match := channelPathPattern.FindStringSubmatch(p)
	if match == nil {
		return "", fmt.Errorf("supplied path does not contain channel name: %s", p)
	}
	return match[1], nil
}
