package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/slugalisk/overrustlelogs/common"
)

// DestinyChat destiny.gg chat client
type DestinyChat struct {
	sync.Mutex
	conn     *websocket.Conn
	dialer   websocket.Dialer
	headers  http.Header
	messages chan *Message
}

// NewDestinyChat new destiny.gg chat client
func NewDestinyChat() *DestinyChat {
	c := &DestinyChat{
		dialer:   websocket.Dialer{HandshakeTimeout: SocketHandshakeTimeout},
		headers:  http.Header{"Origin": []string{common.GetConfig().DestinyGG.OriginURL}},
		messages: make(chan *Message, MessageBufferSize),
	}

	return c
}

// Connect open ws connection
func (c *DestinyChat) Connect() {
	var err error
	c.Lock()
	c.conn, _, err = c.dialer.Dial(common.GetConfig().DestinyGG.SocketURL, c.headers)
	c.Unlock()
	if err != nil {
		log.Printf("error connecting to destiny ws %s", err)
		c.reconnect()
	}
}

func (c *DestinyChat) reconnect() {
	if c.conn != nil {
		c.Lock()
		c.conn.Close()
		c.Unlock()
	}

	time.Sleep(SocketReconnectDelay)
	c.Connect()
}

// Run connect and start message read loop
func (c *DestinyChat) Run() {
	c.Connect()

	for {
		err := c.conn.SetReadDeadline(time.Now().Add(SocketReadTimeout))
		if err != nil {
			c.reconnect()
			continue
		}

		c.Lock()
		_, msg, err := c.conn.ReadMessage()
		c.Unlock()
		if err != nil {
			log.Printf("error reading message %s", err)
			c.reconnect()
			continue
		}

		index := bytes.IndexByte(msg, ' ')
		if index == -1 || len(msg) < index+1 {
			log.Printf("invalid message %s", msg)
			continue
		}

		data := &struct {
			Nick      string
			Data      string
			Timestamp int64
		}{}

		if err := json.Unmarshal(msg[index+1:], data); err != nil {
			continue
		}

		c.messages <- &Message{
			Command: string(msg[:index]),
			Nick:    data.Nick,
			Data:    strings.Replace(data.Data, "\n", " ", -1),
			Time:    time.Now().UTC(),
		}
	}
}

// Messages channel accessor
func (c *DestinyChat) Messages() <-chan *Message {
	return c.messages
}

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
func (d *DestinyLogger) Log(mc <-chan *Message) {
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
			if strings.Contains(m.Data, "subscriber!") || strings.Contains(m.Data, "subscribed on Twitch!") {
				d.writeLine(m.Time, "Subscriber", m.Data)
			}
		case "MSG":
			d.writeLine(m.Time, m.Nick, m.Data)
		}
	}
}

func (d *DestinyLogger) writeLine(timestamp time.Time, nick string, message string) {
	l, err := d.logs.Get(common.GetConfig().DestinyGG.Path + "/" + timestamp.Format("January 2006") + "/" + timestamp.Format("2006-01-02") + ".txt")
	if err != nil {
		log.Printf("error opening log %s", err)
		return
	}
	l.Write(timestamp, nick, message)
}
