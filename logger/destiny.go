package main

import (
	"log"
	"strings"
	"time"

	"github.com/slugalisk/overrustlelogs/common"
)

// DestinyLogger logger
type DestinyLogger struct {
	logs *ChatLogs
}

// NewDestinyLogger instantiates destiny chat logger
func NewDestinyLogger(logs *ChatLogs) *DestinyLogger {
	return &DestinyLogger{
		logs: logs,
	}
}

// Log starts logging loop
func (d *DestinyLogger) Log(mc <-chan *common.Message) {
	var subTrigger bool
	for {
		m := <-mc

		switch m.Command {
		case "BAN":
			d.writeLine(m.Time, "Ban", m.Data+" banned by "+m.Nick)
		case "UNBAN":
			d.writeLine(m.Time, "Ban", m.Data+" unbanned by "+m.Nick)
		case "MUTE":
			d.writeLine(m.Time, "Ban", m.Data+" muted by "+m.Nick)
		case "UNMUTE":
			d.writeLine(m.Time, "Ban", m.Data+" unmuted by "+m.Nick)
		case "BROADCAST":
			if strings.Contains(m.Data, "subscriber!") || strings.Contains(m.Data, "subscribed on Twitch!") || strings.Contains(m.Data, "has resubscribed! Active for") {
				d.writeLine(m.Time, "Subscriber", m.Data)
				subTrigger = true
			} else if subTrigger {
				d.writeLine(m.Time, "SubscriberMessage", m.Data)
			}
		case "MSG":
			d.writeLine(m.Time, m.Nick, m.Data)
			subTrigger = false
		}
	}
}

func (d *DestinyLogger) writeLine(timestamp time.Time, nick string, message string) {
	l, err := d.logs.Get(common.GetConfig().LogPath + "/Destinygg chatlog/" + timestamp.Format("January 2006") + "/" + timestamp.Format("2006-01-02") + ".txt")
	if err != nil {
		log.Printf("error opening log %s", err)
		return
	}
	l.Write(timestamp, nick, message)
}
