package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// errors
var (
	ErrTwitchAlreadyInChannel = errors.New("already in channel")
	ErrTwitchNotInChannel     = errors.New("not in channel")
)

// TwitchChat twitch chat client
type TwitchChat struct {
	sync.RWMutex
	conn        *websocket.Conn
	dialer      websocket.Dialer
	headers     http.Header
	messages    map[string]chan *Message
	channels    []string
	writeQueue  []string
	writeLock   sync.Mutex
	joinHandler TwitchJoinHandler
	admins      map[string]bool
}

// TwitchJoinHandler called when joining channels
type TwitchJoinHandler func(string, chan *Message)

// NewTwitchChat new twitch chat client
func NewTwitchChat(j TwitchJoinHandler) *TwitchChat {
	c := &TwitchChat{
		dialer:      websocket.Dialer{HandshakeTimeout: SocketHandshakeTimeout},
		headers:     http.Header{"Origin": []string{config.Twitch.OriginURL}},
		messages:    make(map[string]chan *Message, 0),
		channels:    make([]string, 0),
		writeQueue:  make([]string, 0),
		joinHandler: j,
		admins:      make(map[string]bool, len(config.Twitch.Admins)),
	}

	for _, u := range config.Twitch.Admins {
		c.admins[u] = true
	}

	d, err := ioutil.ReadFile(config.Twitch.ChannelListPath)
	if err != nil {
		log.Fatalf("unable to read channels %s", err)
	}
	if err := json.Unmarshal(d, &c.channels); err != nil {
		log.Fatalf("unable to read channels %s", err)
	}

	return c
}

// Connect open ws connection
func (c *TwitchChat) Connect() {
	var err error
	c.Lock()
	c.conn, _, err = c.dialer.Dial(config.Twitch.SocketURL, c.headers)
	c.Unlock()
	if err != nil {
		log.Printf("error connecting to twitch ws %s", err)
		c.reconnect()
	}

	c.send("PASS " + config.Twitch.OAuth)
	c.send("NICK " + config.Twitch.Nick)

	for _, ch := range c.channels {
		c.Join(ch, true)
	}
}

func (c *TwitchChat) reconnect() {
	if c.conn != nil {
		c.Lock()
		c.conn.Close()

		for ch, mc := range c.messages {
			close(mc)
			delete(c.messages, ch)
		}
		c.Unlock()
	}

	time.Sleep(SocketReconnectDelay)
	c.Connect()
}

// Run connect and start message read loop
func (c *TwitchChat) Run() {
	c.Connect()

	messagePattern := regexp.MustCompile(`:(.+)\!.+tmi\.twitch\.tv PRIVMSG #([a-z0-9_-]+) :(.+)`)

	for {
		err := c.conn.SetReadDeadline(time.Now().Add(20 * time.Second))
		if err != nil {
			c.reconnect()
			continue
		}

		c.RLock()
		_, msg, err := c.conn.ReadMessage()
		c.RUnlock()
		if err != nil {
			log.Printf("error reading message %s", err)
			c.reconnect()
			continue
		}

		if strings.Index(string(msg), "PING") == 0 {
			c.send(strings.Replace(string(msg), "PING", "PONG", -1))
			continue
		}

		l := messagePattern.FindAllStringSubmatch(string(msg), -1)
		for _, v := range l {
			c.RLock()
			mc, ok := c.messages[strings.ToLower(v[2])]
			c.RUnlock()
			if !ok {
				continue
			}

			data := strings.TrimSpace(v[3])
			data = strings.Replace(data, "ACTION", "/me", -1)
			data = strings.Replace(data, "", "", -1)
			m := &Message{
				Command: "MSG",
				Nick:    v[1],
				Data:    data,
				Time:    time.Now(),
			}

			c.runCommand(v[2], m)

			select {
			case mc <- m:
			default:
			}
		}
	}
}

func (c *TwitchChat) runCommand(source string, m *Message) {
	if _, ok := c.admins[m.Nick]; ok && m.Command == "MSG" {
		d := strings.Split(m.Data, " ")
		ld := strings.Split(strings.ToLower(m.Data), " ")

		if strings.EqualFold(d[0], "!join") {
			if err := c.Join(ld[1], false); err == nil {
				c.send("PRIVMSG #" + source + " :Logging " + ld[1])
			} else {
				c.send("PRIVMSG #" + source + " :Already logging " + ld[1])
			}
		} else if strings.EqualFold(d[0], "!leave") {
			if err := c.Leave(ld[1]); err == nil {
				c.send("PRIVMSG #" + source + " :Leaving " + ld[1])
			} else {
				c.send("PRIVMSG #" + source + " :Not logging " + ld[1])
			}
		} else if strings.EqualFold(d[0], "!channels") {
			c.send("PRIVMSG #" + source + " :Logging " + strings.Join(c.channels, ", "))
		}
	}
}

func (c *TwitchChat) send(m string) {
	c.writeLock.Lock()
	c.RLock()
	err := c.conn.WriteMessage(1, []byte(m+"\r\n"))
	c.RUnlock()
	if err == nil {
		time.Sleep(500 * time.Millisecond)
	}
	c.writeLock.Unlock()
	if err != nil {
		log.Printf("error sending message %s", err)
		c.reconnect()
	}
}

// Join channel
func (c *TwitchChat) Join(ch string, init bool) error {
	ch = strings.ToLower(ch)
	c.Lock()
	_, ok := c.messages[ch]
	if !ok {
		c.messages[ch] = make(chan *Message, MessageBufferSize)
	}
	c.Unlock()
	if ok {
		return ErrTwitchAlreadyInChannel
	}
	c.send("JOIN #" + ch)
	c.Lock()
	if messages, ok := c.messages[ch]; ok {
		c.joinHandler(ch, messages)
	}
	c.Unlock()
	if init {
		return nil
	}
	return c.saveChannels()
}

// Leave channel
func (c *TwitchChat) Leave(ch string) error {
	ch = strings.ToLower(ch)
	c.Lock()
	_, ok := c.messages[ch]
	c.Unlock()
	if !ok {
		return ErrTwitchNotInChannel
	}
	c.send("PART #" + ch)
	c.Lock()
	delete(c.messages, ch)
	c.Unlock()
	return c.saveChannels()
}

func (c *TwitchChat) saveChannels() error {
	c.Lock()
	defer c.Unlock()
	c.channels = c.channels[:]
	for ch := range c.messages {
		c.channels = append(c.channels, ch)
	}
	f, err := os.Create(config.Twitch.ChannelListPath)
	if err != nil {
		log.Printf("error saving channel list %s", err)
		return err
	}
	data, err := json.Marshal(c.channels)
	if err != nil {
		log.Printf("error saving channel list %s", err)
		return err
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", "\t"); err != nil {
		log.Printf("error saving channel list %s", err)
	}
	f.Write(buf.Bytes())
	f.Close()
	return nil
}

// TwitchLogger logger
type TwitchLogger struct {
	logs    *ChatLogs
	channel string
}

// NewTwitchLogger instantiates twitch chat logger
func NewTwitchLogger(logs *ChatLogs, ch string) *TwitchLogger {
	return &TwitchLogger{
		logs:    logs,
		channel: strings.Title(ch),
	}
}

// Log starts logging loop
func (t *TwitchLogger) Log(mc <-chan *Message) {
	for {
		m, ok := <-mc
		if !ok {
			return
		}

		if m.Command == "MSG" {
			t.writeLine(m.Time, m.Time.Format("2006-01-02")+".txt", m.Nick+": "+m.Data)
			t.writeLine(m.Time, "userlogs/"+m.NickPath()+".txt", m.Nick+": "+m.Data)
		}
	}
}

func (t *TwitchLogger) writeLine(timestamp time.Time, path string, line string) {
	l, err := t.logs.Get(config.Twitch.Path + "/" + t.channel + " chatlog/" + timestamp.Format("January 2006") + "/" + path)
	if err != nil {
		log.Printf("error opening log %s", err)
		return
	}
	l.Write(timestamp, line)
}
