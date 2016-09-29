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

// Destiny destiny.gg chat client
type Destiny struct {
	connLock sync.RWMutex
	conn     *websocket.Conn
	dialer   websocket.Dialer
	headers  http.Header
	messages chan *Message
	stopped  bool
}

// NewDestiny new destiny.gg chat client
func NewDestiny() *Destiny {
	c := &Destiny{
		dialer: websocket.Dialer{HandshakeTimeout: HandshakeTimeout},
		headers: http.Header{
			"Origin": []string{GetConfig().DestinyGG.OriginURL},
			"Cookie": []string{GetConfig().DestinyGG.Cookie},
		},
		messages: make(chan *Message, MessageBufferSize),
	}

	return c
}

// Connect open ws connection
func (c *Destiny) connect() {
	var err error
	c.connLock.Lock()
	c.conn, _, err = c.dialer.Dial(GetConfig().DestinyGG.SocketURL, c.headers)
	c.connLock.Unlock()
	if err != nil {
		log.Printf("error connecting to destiny ws %s", err)
		c.reconnect()
		return
	}
	log.Printf("connected to destiny ws")
}

func (c *Destiny) reconnect() {
	if c.conn != nil {
		c.connLock.Lock()
		c.conn.Close()
		c.connLock.Unlock()
	}
	time.Sleep(SocketReconnectDelay)
	c.connect()
}

// Run connect and start message read loop
func (c *Destiny) Run() {
	c.connect()

	for {
		if c.stopped {
			close(c.messages)
			return
		}
		err := c.conn.SetReadDeadline(time.Now().UTC().Add(SocketReadTimeout))
		if err != nil {
			log.Println("SetReadDeadline triggered, reconnecting")
			c.reconnect()
		}

		c.connLock.Lock()
		_, msg, err := c.conn.ReadMessage()
		c.connLock.Unlock()
		if err != nil {
			log.Printf("error reading from websocket %s", err)
			c.reconnect()
			continue
		}

		index := bytes.IndexByte(msg, ' ')
		if index == -1 || len(msg) < index+1 {
			log.Printf("invalid message %s", msg)
			continue
		}

		if strings.Index(string(msg), "PING") == 0 {
			c.send("PONG", map[string]string{"timestamp": "yee"})
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

		select {
		case c.messages <- &Message{
			Command: string(msg[:index]),
			Channel: "Destinygg",
			Nick:    data.Nick,
			Data:    strings.Replace(data.Data, "\n", " ", -1),
			Time:    time.Unix(data.Timestamp/1000, 0).UTC(),
		}:
		default:
		}
	}
}

// Stop ...
func (c *Destiny) Stop() {
	c.stopped = true
	if c.conn != nil {
		c.connLock.Lock()
		c.conn.Close()
		c.connLock.Unlock()
	}
}

// Messages channel accessor
func (c *Destiny) Messages() <-chan *Message {
	return c.messages
}

func (c *Destiny) send(command string, msg map[string]string) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	buf := bytes.NewBuffer([]byte{})
	buf.WriteString(command)
	buf.WriteString(" ")
	buf.Write(data)
	c.connLock.RLock()
	s := time.Now()
	if err := c.conn.WriteMessage(websocket.TextMessage, buf.Bytes()); err != nil {
		log.Printf("error sending message %s", err)
		log.Println("wut y")
		c.connLock.RUnlock()
		c.reconnect()
		return err
	}
	log.Println(time.Since(s))
	c.connLock.RUnlock()
	return nil
}

// Message send message
func (c *Destiny) Message(ch, payload string) error {
	return c.send("MSG", map[string]string{"data": payload})
}

// Whisper send private message
func (c *Destiny) Whisper(nick, data string) error {
	return c.send("PRIVMSG", map[string]string{
		"nick": nick,
		"data": data,
	})
}
