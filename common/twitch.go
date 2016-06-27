package common

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// const ...
const (
	TwitchReadTimeout       = 30 * time.Minute
	TwitchSocketReadTimeout = 30 * time.Second
)

// errors
var (
	ErrAlreadyInChannel = errors.New("already in channel")
	ErrNotInChannel     = errors.New("not in channel")
	ErrChannelNotValid  = errors.New("not a valid channel")
)

// TwitchChat twitch chat client
type TwitchChat struct {
	sendLock     sync.Mutex
	connLock     sync.RWMutex
	conn         *websocket.Conn
	dialer       websocket.Dialer
	headers      http.Header
	messagesLock sync.RWMutex
	messages     map[string]chan *Message
	channelsLock sync.Mutex
	channels     []string
	joinHandler  TwitchJoinHandler
	admins       map[string]bool
	stopped      bool
	evicted      chan string
}

// TwitchJoinHandler called when joining channels
type TwitchJoinHandler func(string, chan *Message)

// NewTwitchChat new twitch chat client
func NewTwitchChat(j TwitchJoinHandler) *TwitchChat {
	c := &TwitchChat{
		dialer:      websocket.Dialer{HandshakeTimeout: SocketHandshakeTimeout},
		headers:     http.Header{"Origin": []string{GetConfig().Twitch.OriginURL}},
		messages:    make(map[string]chan *Message, 0),
		channels:    make([]string, 0),
		joinHandler: j,
		admins:      make(map[string]bool, len(GetConfig().Twitch.Admins)),
		evicted:     make(chan string, 0),
	}

	for _, u := range GetConfig().Twitch.Admins {
		c.admins[u] = true
	}

	d, err := ioutil.ReadFile(GetConfig().Twitch.ChannelListPath)
	if err != nil {
		log.Fatalf("unable to read channels %s", err)
	}
	if err := json.Unmarshal(d, &c.channels); err != nil {
		log.Fatalf("unable to read channels %s", err)
	}
	sort.Strings(c.channels)
	go c.runEvictHandler()
	return c
}

// Connect open ws connection
func (c *TwitchChat) Connect() {
	var err error
	c.connLock.RLock()
	c.conn, _, err = c.dialer.Dial(GetConfig().Twitch.SocketURL, c.headers)
	c.connLock.RUnlock()
	if err != nil {
		log.Printf("error connecting to twitch ws %s", err)
		c.reconnect()
	}

	c.send("PASS " + GetConfig().Twitch.OAuth)
	c.send("NICK " + GetConfig().Twitch.Nick)

	for _, ch := range c.channels {
		err := c.Join(ch)
		if err != nil {
			log.Println(ch, err)
		}
	}
}

func (c *TwitchChat) reconnect() {
	if c.conn != nil {
		c.connLock.Lock()
		c.conn.Close()
		c.connLock.Unlock()
	}
	c.cleanupMessages()

	time.Sleep(SocketReconnectDelay)
	c.Connect()
}

func (c *TwitchChat) cleanupMessages() {
	c.messagesLock.Lock()
	defer c.messagesLock.Unlock()
	if len(c.messages) == 0 {
		return
	}
	for ch, mc := range c.messages {
		close(mc)
		delete(c.messages, ch)
	}
}

// Run connect and start message read loop
func (c *TwitchChat) Run() {
	c.Connect()

	messagePattern := regexp.MustCompile(`:(.+)\!.+tmi\.twitch\.tv PRIVMSG #([a-z0-9_-]+) :(.+)`)
	w := NewTimeWheel(TwitchReadTimeout, time.Second, c.evict)
	for _, ch := range c.channels {
		w.Update(strings.ToLower(ch))
	}

	for {
		err := c.conn.SetReadDeadline(time.Now().Add(TwitchSocketReadTimeout))
		if err != nil {
			c.reconnect()
			continue
		}

		c.connLock.RLock()
		_, msg, err := c.conn.ReadMessage()
		c.connLock.RUnlock()
		if c.stopped {
			return
		}
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
			w.Update(v[2])
			c.messagesLock.RLock()
			mc, ok := c.messages[strings.ToLower(v[2])]
			c.messagesLock.RUnlock()
			if !ok {
				continue
			}

			data := strings.TrimSpace(v[3])
			data = strings.Replace(data, "ACTION", "/me", -1)
			data = strings.Replace(data, "", "", -1)
			m := &Message{
				Command: "MSG",
				Channel: v[2],
				Nick:    v[1],
				Data:    data,
				Time:    time.Now().UTC(),
			}

			if strings.EqualFold(v[2], GetConfig().Twitch.CommandChannel) {
				c.runCommand(m)
			}

			select {
			case mc <- m:
			default:
			}
		}
	}
}

func (c *TwitchChat) runCommand(m *Message) {
	if _, ok := c.admins[m.Nick]; !ok && m.Command != "MSG" {
		return
	}
	ld := strings.Split(strings.ToLower(m.Data), " ")
	switch ld[0] {
	case "!join":
		err := c.Join(ld[1])
		switch err {
		case nil:
			c.send(fmt.Sprintf("PRIVMSG #%s :Logging %s", m.Channel, ld[1]))
		case ErrChannelNotValid:
			c.send(fmt.Sprintf("PRIVMSG #%s :Channel doesn't exist!", m.Channel))
		case ErrAlreadyInChannel:
			c.send(fmt.Sprintf("PRIVMSG #%s :Already logging %s", m.Channel, ld[1]))
		default:
		}
	case "!leave":
		err := c.Leave(ld[1])
		switch err {
		case nil:
			c.send(fmt.Sprintf("PRIVMSG #%s :Leaving %s", m.Channel, ld[1]))
		case ErrNotInChannel:
			c.send(fmt.Sprintf("PRIVMSG #%s :Not logging %s", m.Channel, ld[1]))
		default:
		}
	case "!channels":
		c.send(fmt.Sprintf("PRIVMSG #%s :Logging %d channels.", m.Channel, len(c.channels)))
	}
}

func (c *TwitchChat) send(m string) {
	c.connLock.RLock()
	c.sendLock.Lock()
	err := c.conn.WriteMessage(websocket.TextMessage, []byte(m+"\r\n"))
	c.sendLock.Unlock()
	c.connLock.RUnlock()
	if err == nil {
		time.Sleep(SocketWriteDebounce)
	}
	if err != nil {
		log.Printf("error sending message %s", err)
		c.reconnect()
	}
}

// Join channel
func (c *TwitchChat) Join(ch string) error {
	ch = strings.ToLower(ch)
	if !channelExists(ch) {
		c.removeChannel(ch)
		return ErrChannelNotValid
	}
	c.messagesLock.Lock()
	if _, ok := c.messages[ch]; ok {
		c.messagesLock.Unlock()
		log.Println("already in channel", ch)
		return ErrAlreadyInChannel
	}
	c.messages[ch] = make(chan *Message, MessageBufferSize)
	c.messagesLock.Unlock()
	c.send("JOIN #" + ch)
	if messages, ok := c.messages[ch]; ok {
		go c.joinHandler(ch, messages)
	}
	// if the channel is already in the list then there is no need to save the channels again
	c.channelsLock.Lock()
	for _, channel := range c.channels {
		if strings.EqualFold(channel, ch) {
			c.channelsLock.Unlock()
			return nil
		}
	}
	c.channels = append(c.channels, ch)
	c.channelsLock.Unlock()
	return c.saveChannels()
}

// Leave channel
func (c *TwitchChat) Leave(ch string) error {
	ch = strings.ToLower(ch)
	c.messagesLock.Lock()
	m, ok := c.messages[ch]
	c.messagesLock.Unlock()
	if !ok {
		return ErrNotInChannel
	}
	c.send("PART #" + ch)
	c.messagesLock.Lock()
	delete(c.messages, ch)
	close(m)
	c.messagesLock.Unlock()
	if err := c.removeChannel(ch); err != nil {
		return nil
	}
	return c.saveChannels()
}

func (c *TwitchChat) removeChannel(ch string) error {
	c.channelsLock.Lock()
	defer c.channelsLock.Unlock()
	for i, channel := range c.channels {
		if strings.EqualFold(channel, ch) {
			c.channels = append(c.channels[:i], c.channels[i+1:]...)
			return nil
		}
	}
	return errors.New("didn't find " + ch)
}

func (c *TwitchChat) evict(ch string) {
	ch = strings.ToLower(ch)
	c.messagesLock.RLock()
	_, ok := c.messages[ch]
	c.messagesLock.RUnlock()
	if ok {
		log.Printf("evicted %s", ch)
		err := c.Leave(ch)
		if err != nil {
			log.Println(err)
		}
		c.evicted <- ch
	}
}

func (c *TwitchChat) runEvictHandler() {
	for {
		ch := <-c.evicted
		c.Join(ch)
	}
}

// Empty ...
func (c *TwitchChat) Empty(ch string) bool {
	c.messagesLock.RLock()
	defer c.messagesLock.RUnlock()
	return len(c.messages[ch]) == 0
}

// Stop ...
func (c *TwitchChat) Stop() {
	c.connLock.Lock()
	if c.stopped {
		c.connLock.Unlock()
		return
	}
	c.stopped = true
	c.connLock.Unlock()

	c.messagesLock.RLock()
	m := make([]string, 0, len(c.messages))
	for ch := range c.messages {
		m = append(m, ch)
	}
	c.messagesLock.RUnlock()
	for _, ch := range m {
		c.evict(ch)
	}
	close(c.evicted)

	c.connLock.Lock()
	c.conn.Close()
	c.connLock.Unlock()
	c.saveChannels()
}

// channelExists
func channelExists(ch string) bool {
	u, err := url.Parse("https://api.twitch.tv/kraken/users/" + ch)
	if err != nil {
		log.Panicf("error parsing twitch metadata endpoint url %s", err)
	}
	req := &http.Request{
		Header: http.Header{
			"Client-ID": []string{GetConfig().Twitch.ClientID},
		},
		URL: u,
	}
	client := http.Client{}
	res, _ := client.Do(req)
	defer res.Body.Close()
	return res.StatusCode == http.StatusOK
}

func (c *TwitchChat) saveChannels() error {
	f, err := os.Create(GetConfig().Twitch.ChannelListPath)
	if err != nil {
		log.Printf("error saving channel list %s", err)
		return err
	}
	defer f.Close()
	sort.Strings(c.channels)
	data, err := json.Marshal(c.channels)
	if err != nil {
		log.Printf("error saving channel list %s", err)
		return err
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", "\t"); err != nil {
		log.Printf("error saving channel list %s", err)
		return err
	}
	f.Write(buf.Bytes())
	return nil
}
