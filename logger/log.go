package main

import (
	"log"
	"strings"
	"time"

	"github.com/slugalisk/overrustlelogs/common"
)

// Logger logger
type Logger struct {
	logs    *ChatLogs
	channel string
}

// NewLogger instantiates destiny chat logger
func NewLogger(logs *ChatLogs) *Logger {
	return &Logger{
		logs: logs,
	}
}

// DestinyLog starts logging loop
func (l *Logger) DestinyLog(mc <-chan *common.Message) {
	var subTrigger bool
	for {
		m, ok := <-mc
		if !ok {
			return
		}

		switch m.Command {
		case "BAN":
			l.writeLine(m.Time, m.Channel, "Ban", m.Data+" banned by "+m.Nick)
		case "UNBAN":
			l.writeLine(m.Time, m.Channel, "Ban", m.Data+" unbanned by "+m.Nick)
		case "MUTE":
			l.writeLine(m.Time, m.Channel, "Ban", m.Data+" muted by "+m.Nick)
		case "UNMUTE":
			l.writeLine(m.Time, m.Channel, "Ban", m.Data+" unmuted by "+m.Nick)
		case "BROADCAST":
			if strings.Contains(m.Data, "subscriber!") || strings.Contains(m.Data, "subscribed on Twitch!") || strings.Contains(m.Data, "has resubscribed! Active for") {
				l.writeLine(m.Time, m.Channel, "Subscriber", m.Data)
				subTrigger = true
			} else if subTrigger {
				l.writeLine(m.Time, m.Channel, "SubscriberMessage", m.Data)
			} else {
				l.writeLine(m.Time, m.Channel, "Broadcast", m.Data)
			}
		case "MSG":
			l.writeLine(m.Time, m.Channel, m.Nick, m.Data)
			subTrigger = false
		}
	}
}

// TwitchLog starts logging loop
func (l *Logger) TwitchLog(mc <-chan *common.Message) {
	for {
		m, ok := <-mc
		if !ok {
			return
		}

		if m.Command == "MSG" {
			l.writeLine(m.Time, m.Channel, m.Nick, m.Data)
		}
	}
}

func (l *Logger) writeLine(timestamp time.Time, channel, nick, message string) {
	logs, err := l.logs.Get(common.GetConfig().LogPath + "/" + strings.Title(channel) + " chatlog/" + timestamp.Format("January 2006") + "/" + timestamp.Format("2006-01-02") + ".txt")
	if err != nil {
		log.Printf("error opening log %s", err)
		return
	}
	logs.Write(timestamp, nick, message)
}
