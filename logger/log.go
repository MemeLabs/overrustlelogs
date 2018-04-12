package main

import (
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/MemeLabs/overrustlelogs/common"
)

// Logger logger
type Logger struct {
	logs *ChatLogs
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
	giftRegex := regexp.MustCompile("^[a-zA-Z0-9_]+ gifted [a-zA-Z0-9_]+ a Tier (I|II|II|IV) subscription!")

loop:
	for m := range mc {
		switch m.Type {
		case "BAN":
			l.writeLine(m.Time, m.Channel, "Ban", fmt.Sprintf("%s banned by %s", m.Data, m.Nick))
		case "UNBAN":
			l.writeLine(m.Time, m.Channel, "Ban", fmt.Sprintf("%s unbanned by %s", m.Data, m.Nick))
		case "MUTE":
			l.writeLine(m.Time, m.Channel, "Ban", fmt.Sprintf("%s muted by %s", m.Data, m.Nick))
		case "UNMUTE":
			l.writeLine(m.Time, m.Channel, "Ban", fmt.Sprintf("%s unmuted by %s", m.Data, m.Nick))
		case "BROADCAST":
			subMessages := []string{"subscriber!", "subscribed on Twitch!", "has resubscribed! Active for", "has resubscribed on Twitch! active"}

			for _, smsg := range subMessages {
				if strings.Contains(m.Data, smsg) {
					l.writeLine(m.Time, m.Channel, "Subscriber", m.Data)
					subTrigger = !subTrigger
					continue loop
				}
			}
			if giftRegex.MatchString(m.Data) {
				l.writeLine(m.Time, m.Channel, "Subscriber", m.Data)
				subTrigger = !subTrigger
				continue loop
			}

			if subTrigger {
				l.writeLine(m.Time, m.Channel, "SubscriberMessage", m.Data)
				subTrigger = !subTrigger
				continue
			}
			l.writeLine(m.Time, m.Channel, "Broadcast", m.Data)
		case "MSG":
			l.writeLine(m.Time, m.Channel, m.Nick, m.Data)
			subTrigger = false
		}
	}
}

// TwitchLog starts logging loop
func (l *Logger) TwitchLog(mc <-chan *common.Message) {
	for m := range mc {
		if m.Type == "MSG" {
			l.writeLine(m.Time, m.Channel, m.Nick, m.Data)
		}
	}
}

func (l *Logger) writeLine(timestamp time.Time, channel, nick, message string) {
	logs, err := l.logs.Get(filepath.Join(common.GetConfig().LogPath, strings.Title(channel)+" chatlog", timestamp.Format("January 2006"), timestamp.Format("2006-01-02")+".txt"))
	if err != nil {
		log.Printf("error opening log %s", err)
		return
	}
	logs.Write(timestamp, nick, message)
}
