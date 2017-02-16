package common

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// errors
var (
	ErrAlreadyInChannel = errors.New("already in channel")
	ErrNotInChannel     = errors.New("not in channel")
	ErrChannelNotValid  = errors.New("not a valid channel")
)

// Twitch twitch chat client
type Twitch struct {
	sendLock       sync.Mutex
	connLock       sync.RWMutex
	conn           *websocket.Conn
	dialer         websocket.Dialer
	headers        http.Header
	ChLock         sync.Mutex
	channels       []string
	messages       chan *Message
	MessagePattern *regexp.Regexp
	stopped        bool
	debug          bool
}

// NewTwitch new twitch chat client
func NewTwitch() *Twitch {
	return &Twitch{
		dialer:         websocket.Dialer{HandshakeTimeout: HandshakeTimeout},
		headers:        http.Header{"Origin": []string{GetConfig().Twitch.OriginURL}},
		channels:       make([]string, 0),
		messages:       make(chan *Message, MessageBufferSize),
		MessagePattern: regexp.MustCompile(`:(.+)\!.+tmi\.twitch\.tv PRIVMSG #([a-z0-9_-]+) :(.+)`),
	}
}

func (c *Twitch) connect() {
	var err error
	c.connLock.Lock()
	c.conn, _, err = c.dialer.Dial(GetConfig().Twitch.SocketURL, c.headers)
	c.connLock.Unlock()
	if err != nil {
		log.Printf("error connecting to twitch ws %s", err)
		c.reconnect()
		return
	}

	conf := GetConfig()
	if conf.Twitch.OAuth == "" || conf.Twitch.Nick == "" {
		log.Println("missing OAuth or Nick, using justinfan654 as login data")
		conf.Twitch.OAuth = "justinfan659"
		conf.Twitch.Nick = "justinfan659"
	}

	c.send("PASS " + conf.Twitch.OAuth)
	c.send("NICK " + conf.Twitch.Nick)

	for _, ch := range c.channels {
		ch = strings.ToLower(ch)
		log.Printf("joining %s", ch)
		err := c.send("JOIN #" + ch)
		if err != nil {
			log.Println("failed to join", ch, "after freshly re/connecting to the websocket")
		}
	}
}

func (c *Twitch) reconnect() {
	c.connLock.Lock()
	if c.conn != nil {
		c.conn.Close()
	}
	c.connLock.Unlock()

	time.Sleep(SocketReconnectDelay)
	c.connect()
}

// Run connect and start message read loop
func (c *Twitch) Run() {
	c.connect()
	go c.rejoinHandler()
	go func() {
		defer close(c.messages)
		for !c.stopped {
			c.connLock.Lock()
			err := c.conn.SetReadDeadline(time.Now().Add(SocketReadTimeout))
			c.connLock.Unlock()
			if err != nil {
				log.Printf("error setting the ReadDeadline %s", err)
				c.reconnect()
				continue
			}

			c.connLock.Lock()
			_, msg, err := c.conn.ReadMessage()
			c.connLock.Unlock()
			if err != nil {
				log.Printf("error reading message %s", err)
				c.reconnect()
				continue
			}

			if strings.Index(string(msg), "PING") == 0 {
				err := c.send(strings.Replace(string(msg), "PING", "PONG", -1))
				if err != nil {
					log.Println("error sending PONG")
					c.reconnect()
				}
				continue
			}

			l := c.MessagePattern.FindAllStringSubmatch(string(msg), -1)
			for _, v := range l {
				data := strings.TrimSpace(v[3])
				data = strings.Replace(data, "ACTION", "/me", -1)
				data = strings.Replace(data, "", "", -1)
				m := &Message{
					Type:    "MSG",
					Channel: v[2],
					Nick:    v[1],
					Data:    data,
					Time:    time.Now().UTC(),
				}

				select {
				case c.messages <- m:
				default:
					log.Println("error messages channel full :(")
				}
			}
		}
	}()
}

// Channels ...
func (c *Twitch) Channels() []string {
	return c.channels
}

// Messages channel accessor
func (c *Twitch) Messages() <-chan *Message {
	return c.messages
}

// Message send a message to a channel
func (c *Twitch) Message(ch, payload string) error {
	return c.send(fmt.Sprintf("PRIVMSG #%s :%s", ch, payload))
}

func (c *Twitch) send(m string) error {
	c.sendLock.Lock()
	err := c.conn.SetWriteDeadline(time.Now().Add(SocketWriteTimeout))
	c.sendLock.Unlock()
	if err != nil {
		return fmt.Errorf("error setting SetWriteDeadline %s", err)
	}

	c.sendLock.Lock()
	err = c.conn.WriteMessage(websocket.TextMessage, []byte(m+"\r\n"))
	c.sendLock.Unlock()
	if err != nil {
		return fmt.Errorf("error sending message %s", err)
	}
	time.Sleep(SocketWriteDebounce)
	return nil
}

// Join channel
func (c *Twitch) Join(ch string) error {
	ch = strings.ToLower(ch)
	err := c.send("JOIN #" + ch)
	if err != nil {
		c.reconnect()
		return err
	}
	c.ChLock.Lock()
	defer c.ChLock.Unlock()
	if inSlice(c.channels, ch) {
		return ErrAlreadyInChannel
	}
	c.channels = append(c.channels, ch)
	return nil
}

// Leave channel
func (c *Twitch) Leave(ch string) error {
	ch = strings.ToLower(ch)
	err := c.send("PART #" + ch)
	if err != nil {
		log.Printf("error leaving channel: %s", err)
		c.reconnect()
	}
	return c.removeChannel(ch)
}

func (c *Twitch) removeChannel(ch string) error {
	c.ChLock.Lock()
	defer c.ChLock.Unlock()
	sort.Strings(c.channels)
	i := sort.SearchStrings(c.channels, ch)
	if i < len(c.channels) && c.channels[i] == ch {
		c.channels = append(c.channels[:i], c.channels[i+1:]...)
		return nil
	}
	return ErrNotInChannel
}

func (c *Twitch) rejoinHandler() {
	const interval = 1 * time.Hour
	for range time.Tick(interval) {
		if c.stopped {
			return
		}
		c.ChLock.Lock()
		for _, ch := range c.channels {
			ch = strings.ToLower(ch)
			log.Printf("rejoining %s\n", ch)
			err := c.send("JOIN #" + ch)
			if err != nil {
				log.Println(err)
				break
			}
		}
		c.ChLock.Unlock()
	}
}

// Stop stops the chats
func (c *Twitch) Stop() {
	c.stopped = true
	c.sendLock.Lock()
	c.ChLock.Lock()
	c.connLock.Lock()
	if c.conn != nil {
		c.conn.Close()
	}
	c.connLock.Unlock()
	c.ChLock.Unlock()
	c.sendLock.Unlock()
}

func inSlice(s []string, v string) bool {
	for _, sv := range s {
		if strings.EqualFold(sv, v) {
			return true
		}
	}
	return false
}
