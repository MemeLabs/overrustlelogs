package chat

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/slugalisk/overrustlelogs/common"
)

// errors
var (
	errAlreadyInChannel = errors.New("already in channel")
	errNotInChannel     = errors.New("not in channel")
	errChannelNotValid  = errors.New("not a valid channel")
)

// Twitch twitch chat client
type Twitch struct {
	sendLock       sync.Mutex
	connLock       sync.RWMutex
	conn           *websocket.Conn
	dialer         websocket.Dialer
	headers        http.Header
	chLock         sync.Mutex
	channels       []string
	messages       chan *common.Message
	MessagePattern *regexp.Regexp
	stopped        bool
	join           chan string
	debug          bool
}

// NewTwitch new twitch chat client
func NewTwitch() *Twitch {
	c := &Twitch{
		dialer:   websocket.Dialer{HandshakeTimeout: common.HandshakeTimeout},
		headers:  http.Header{"Origin": []string{common.GetConfig().Twitch.OriginURL}},
		channels: make([]string, 0),
		messages: make(chan *common.Message, common.MessageBufferSize),
		join:     make(chan string, 10),
	}
	c.MessagePattern = regexp.MustCompile(`:(.+)\!.+tmi\.twitch\.tv PRIVMSG #([a-z0-9_-]+) :(.+)`)
	return c
}

func (c *Twitch) connect() {
	var err error
	c.connLock.Lock()
	c.conn, _, err = c.dialer.Dial(common.GetConfig().Twitch.SocketURL, c.headers)
	c.connLock.Unlock()
	if err != nil {
		log.Printf("error connecting to twitch ws %s", err)
		c.reconnect()
	}
	log.Println("sending login data")
	c.send("PASS " + common.GetConfig().Twitch.OAuth)
	c.send("NICK " + common.GetConfig().Twitch.Nick)
	log.Println("finished sending login data")
	for _, ch := range c.channels {
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

	time.Sleep(common.SocketReconnectDelay)
	c.connect()
}

// Debug ...
func (c *Twitch) Debug(b bool) {
	c.debug = b
}

// Run connect and start message read loop
func (c *Twitch) Run() {
	c.connect()
	go c.rejoinHandler()

	for {
		if c.stopped {
			return
		}
		err := c.conn.SetReadDeadline(time.Now().Add(common.SocketReadTimeout))
		if err != nil {
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
			c.send(strings.Replace(string(msg), "PING", "PONG", -1))
			continue
		}

		l := c.MessagePattern.FindAllStringSubmatch(string(msg), -1)
		for _, v := range l {

			data := strings.TrimSpace(v[3])
			data = strings.Replace(data, "ACTION", "/me", -1)
			data = strings.Replace(data, "", "", -1)
			m := &common.Message{
				Command: "MSG",
				Channel: v[2],
				Nick:    v[1],
				Data:    data,
				Time:    time.Now().UTC(),
			}
			if c.debug {
				log.Println(m)
			}
			select {
			case c.messages <- m:
			default:
				log.Println("discarded message :(")
			}
		}
	}
}

// Channels ...
func (c *Twitch) Channels() []string {
	return c.channels
}

// Messages channel accessor
func (c *Twitch) Messages() <-chan *common.Message {
	return c.messages
}

// Message send a message to a channel
func (c *Twitch) Message(ch, payload string) error {
	return c.send(fmt.Sprintf("PRIVMSG #%s :%s", ch, payload))
}

// Whisper ...
func (c *Twitch) Whisper(nick, payload string) error {
	// NOTE: implement
	return nil
}

func (c *Twitch) send(m string) error {
	c.conn.SetWriteDeadline(time.Now().Add(common.SocketWriteTimeout))
	c.sendLock.Lock()
	err := c.conn.WriteMessage(websocket.TextMessage, []byte(m+"\r\n"))
	c.sendLock.Unlock()
	if err != nil {
		return fmt.Errorf("error sending message %s", err)
	}
	time.Sleep(common.SocketWriteDebounce)
	return nil
}

// Join channel
func (c *Twitch) Join(ch string) error {
	ch = strings.ToLower(ch)
	err := c.send("JOIN #" + ch)
	if err != nil {
		return fmt.Errorf("failed to join %s", ch)
	}
	for _, channel := range c.channels {
		if strings.EqualFold(channel, ch) {
			log.Println("wut")
			return nil
		}
	}
	c.chLock.Lock()
	c.channels = append(c.channels, ch)
	c.chLock.Unlock()
	return nil
}

// Leave channel
func (c *Twitch) Leave(ch string) error {
	ch = strings.ToLower(ch)
	c.send("PART #" + ch)
	return c.removeChannel(ch)
}

func (c *Twitch) removeChannel(ch string) error {
	c.chLock.Lock()
	defer c.chLock.Unlock()
	for i, channel := range c.channels {
		if strings.EqualFold(channel, ch) {
			c.channels = append(c.channels[:i], c.channels[i+1:]...)
			return nil
		}
	}
	return errNotInChannel
}

func (c *Twitch) rejoinHandler() {
	tick := time.NewTicker(common.TwitchMessageTimeout)
	for {
		<-tick.C
		log.Println("it's channel rejoin triggered------------------------------------------------------")
		for _, ch := range c.channels {
			ch = strings.ToLower(ch)
			log.Printf("rejoining %s\n", ch)
			err := c.send("JOIN #" + ch)
			if err != nil {
				log.Println(err)
			}
		}
	}
}

// Stop ...
func (c *Twitch) Stop() {
	c.stopped = true
	c.sendLock.Lock()
	c.connLock.Lock()
	if c.conn != nil {
		c.conn.Close()
	}
	c.connLock.Unlock()
	c.sendLock.Unlock()
	close(c.join)
}
