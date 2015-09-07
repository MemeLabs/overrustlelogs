package main

import (
	"log"
	"strings"
	"time"

	"github.com/slugalisk/overrustlelogs/common"
)

// TwitchLogger logger
type TwitchLogger struct {
	logs    *ChatLogs
	channel string
}

// NewTwitchLogger instantiates twitch chat logger
func NewTwitchLogger(logs *ChatLogs, ch string) *TwitchLogger {
	return &TwitchLogger{
		logs:    logs,
		channel: strings.Title(ch),
	}
}

// Log starts logging loop
func (t *TwitchLogger) Log(mc <-chan *common.Message) {
	for {
		m, ok := <-mc
		if !ok {
			return
		}

		if m.Command == "MSG" {
			t.writeLine(m.Time, m.Nick, m.Data)
		}
	}
}

func (t *TwitchLogger) writeLine(timestamp time.Time, nick string, message string) {
	l, err := t.logs.Get(common.GetConfig().LogPath + "/" + t.channel + " chatlog/" + timestamp.Format("January 2006") + "/" + timestamp.Format("2006-01-02") + ".txt")
	if err != nil {
		log.Printf("error opening log %s", err)
		return
	}
	l.Write(timestamp, nick, message)
}
