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
	connLock      sync.Mutex
	conn          *websocket.Conn
	lastMessageMu sync.RWMutex
	lastMessage   time.Time
	messages      chan *Message
	quit          chan struct{}
}

// NewDestiny new destiny.gg chat client
func NewDestiny() *Destiny {
	return &Destiny{
		messages: make(chan *Message, MessageBufferSize),
		quit:     make(chan struct{}, 2),
	}
}

// Connect open ws connection
func (c *Destiny) connect() {
	dialer := websocket.Dialer{HandshakeTimeout: HandshakeTimeout}
	header := http.Header{
		"Origin": []string{GetConfig().DestinyGG.OriginURL},
		"Cookie": []string{GetConfig().DestinyGG.Cookie},
	}
	var err error
	c.connLock.Lock()
	c.conn, _, err = dialer.Dial(GetConfig().DestinyGG.SocketURL, header)
	c.connLock.Unlock()
	if err != nil {
		log.Printf("error connecting to destiny ws %s", err)
		c.reconnect()
		return
	}
	log.Printf("connected to destiny ws")

	go func() {
		timeout := time.NewTimer(2 * time.Minute)
		for range timeout.C {
			c.lastMessageMu.RLock()
			if time.Now().Sub(c.lastMessage).Minutes() > 2 {
				log.Println("destiny timeout triggered")
				c.lastMessageMu.RUnlock()
				c.reconnect()
				continue
			}
			c.lastMessageMu.RUnlock()
		}
	}()
}

func (c *Destiny) reconnect() {
	c.connLock.Lock()
	if c.conn != nil {
		c.conn.Close()
	}
	c.connLock.Unlock()
	time.Sleep(SocketReconnectDelay)
	c.connect()
}

// Run connect and start message read loop
func (c *Destiny) Run() {
	c.connect()
	defer close(c.messages)
	for {
		select {
		case <-c.quit:
			return
		default:
		}

		err := c.conn.SetReadDeadline(time.Now().Add(SocketReadTimeout))
		if err != nil {
			log.Println("SetReadDeadline triggered, reconnecting")
			c.reconnect()
			continue
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
			err := c.conn.WriteMessage(websocket.TextMessage, bytes.Replace(msg, []byte("PING"), []byte("PONG"), -1))
			if err != nil {
				c.reconnect()
			}
			continue
		}

		data := &struct {
			Nick      string
			Data      string
			Timestamp int64
		}{}

		if err := json.Unmarshal(msg[index+1:], data); err != nil {
			log.Println(string(msg))
			continue
		}

		select {
		case c.messages <- &Message{
			Type:    string(msg[:index]),
			Channel: "Destinygg",
			Nick:    data.Nick,
			Data:    strings.Replace(data.Data, "\n", " ", -1),
			Time:    time.Unix(data.Timestamp/1000, 0).UTC(),
		}:
		default:
		}
		c.lastMessageMu.Lock()
		c.lastMessage = time.Now()
		c.lastMessageMu.Unlock()
	}
}

// Stop ...
func (c *Destiny) Stop() {
	c.quit <- struct{}{}
	c.connLock.Lock()
	if c.conn != nil {
		c.conn.Close()
	}
	c.connLock.Unlock()
}

// Messages channel accessor
func (c *Destiny) Messages() <-chan *Message { return c.messages }

func (c *Destiny) send(command string, msg map[string]string) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	buf.WriteString(command)
	buf.WriteString(" ")
	buf.Write(data)
	if err := c.conn.WriteMessage(websocket.TextMessage, buf.Bytes()); err != nil {
		log.Printf("error sending message %s", err)
		c.reconnect()
		return err
	}
	return nil
}

// Message send message
func (c *Destiny) Message(payload string) error {
	return c.send("MSG", map[string]string{"data": payload})
}

// Whisper send private message
func (c *Destiny) Whisper(nick, data string) error {
	return c.send("PRIVMSG", map[string]string{
		"nick": nick,
		"data": data,
	})
}
