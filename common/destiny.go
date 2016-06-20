package common

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// DestinyChat destiny.gg chat client
type DestinyChat struct {
	sync.RWMutex
	conn     *websocket.Conn
	dialer   websocket.Dialer
	headers  http.Header
	messages chan *Message
}

// NewDestinyChat new destiny.gg chat client
func NewDestinyChat() *DestinyChat {
	c := &DestinyChat{
		dialer: websocket.Dialer{HandshakeTimeout: SocketHandshakeTimeout},
		headers: http.Header{
			"Origin": []string{GetConfig().DestinyGG.OriginURL},
			"Cookie": []string{GetConfig().DestinyGG.Cookie},
		},
		messages: make(chan *Message, MessageBufferSize),
	}

	return c
}

// Connect open ws connection
func (c *DestinyChat) Connect() {
	var err error
	c.Lock()
	c.conn, _, err = c.dialer.Dial(GetConfig().DestinyGG.SocketURL, c.headers)
	c.Unlock()
	if err != nil {
		log.Printf("error connecting to destiny ws %s", err)
		c.reconnect()
		return
	}
	log.Printf("connected to destiny ws")
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
		c.RLock()
		_, msg, err := c.conn.ReadMessage()
		c.RUnlock()
		if err != nil {
			log.Printf("error reading from websocket %s", err)
			c.reconnect()
			continue
		}
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

func (c *DestinyChat) send(command string, msg map[string]string) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	buf := bytes.NewBuffer([]byte{})
	buf.WriteString(command)
	buf.WriteString(" ")
	buf.Write(data)
	c.RLock()
	defer c.RUnlock()
	if err := c.conn.WriteMessage(websocket.TextMessage, buf.Bytes()); err != nil {
		log.Printf("error sending message %s", err)
		c.reconnect()
		return err
	}
	return nil
}

// Write send message
func (c *DestinyChat) Write(data string) error {
	return c.send("MSG", map[string]string{"data": data})
}

// WritePrivate send private message
func (c *DestinyChat) WritePrivate(nick, data string) error {
	return c.send("PRIVMSG", map[string]string{
		"nick": nick,
		"data": data,
	})
}
