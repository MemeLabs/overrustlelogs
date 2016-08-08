package chat

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

// Destiny destiny.gg chat client
type Destiny struct {
	connLock sync.RWMutex
	conn     *websocket.Conn
	dialer   websocket.Dialer
	headers  http.Header
	messages chan *common.Message
	stopped  bool
}

// NewDestiny new destiny.gg chat client
func NewDestiny() *Destiny {
	c := &Destiny{
		dialer: websocket.Dialer{HandshakeTimeout: common.HandshakeTimeout},
		headers: http.Header{
			"Origin": []string{common.GetConfig().DestinyGG.OriginURL},
			"Cookie": []string{common.GetConfig().DestinyGG.Cookie},
		},
		messages: make(chan *common.Message, common.MessageBufferSize),
	}

	return c
}

// Connect open ws connection
func (c *Destiny) connect() {
	var err error
	c.connLock.Lock()
	c.conn, _, err = c.dialer.Dial(common.GetConfig().DestinyGG.SocketURL, c.headers)
	c.connLock.Unlock()
	if err != nil {
		log.Printf("error connecting to destiny ws %s", err)
		c.reconnect()
		return
	}
	log.Printf("connected to destiny ws")
}

func (c *Destiny) reconnect() {
	c.connLock.Lock()
	if c.conn != nil {
		c.conn.Close()
	}
	c.connLock.Unlock()

	time.Sleep(common.SocketReconnectDelay)
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
		err := c.conn.SetReadDeadline(time.Now().UTC().Add(common.SocketReadTimeout))
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
			c.send("PONG", map[string]interface{}{"timestamp": time.Now().UnixNano()})
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
		case c.messages <- &common.Message{
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
	c.connLock.Lock()
	defer c.connLock.Unlock()
	if c.conn != nil {
		c.conn.Close()
	}
	return
}

// Messages channel accessor
func (c *Destiny) Messages() <-chan *common.Message {
	return c.messages
}

func (c *Destiny) send(command string, msg map[string]interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	buf := bytes.NewBuffer([]byte{})
	buf.WriteString(command)
	buf.WriteString(" ")
	buf.Write(data)
	c.connLock.RLock()
	c.conn.SetWriteDeadline(time.Now().Add(common.SocketWriteTimeout))
	if err := c.conn.WriteMessage(websocket.TextMessage, buf.Bytes()); err != nil {
		log.Printf("error sending message %s", err)
		c.connLock.RUnlock()
		c.reconnect()
		return err
	}
	c.connLock.RUnlock()
	return nil
}

// Message send message
func (c *Destiny) Message(ch, payload string) error {
	return c.send("MSG", map[string]interface{}{"data": payload})
}

// Whisper send private message
func (c *Destiny) Whisper(nick, data string) error {
	return c.send("PRIVMSG", map[string]interface{}{
		"nick": nick,
		"data": data,
	})
}
