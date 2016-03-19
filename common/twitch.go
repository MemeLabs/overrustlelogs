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
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// errors
var (
	ErrTwitchAlreadyInChannel      = errors.New("already in channel")
	ErrTwitchNotInChannel          = errors.New("not in channel")
	ErrChannelNotValid             = errors.New("not a valid channel")
	ErrChannelMetadataNotAvailable = errors.New("channel metadata not available")
	ErrCommandChannelFailure       = errors.New("command channel failed")
)

// consts...
const (
	TwitchChannelServerAPI  = "http://tmi.twitch.tv/servers"
	TwitchMaxConnectRetries = 3
	TwitchReadTimeout       = 30 * time.Minute
)

// TwitchChat twitch chat client
type TwitchChat struct {
	sync.Mutex
	messages       map[string]chan *Message
	channels       []string
	channelClients map[string]*twitchSocketClient
	clients        map[string]*twitchSocketClient
	joinHandler    TwitchJoinHandler
	admins         map[string]bool
}

// TwitchJoinHandler called when joining channels
type TwitchJoinHandler func(string, chan *Message)

// NewTwitchChat new twitch chat client
func NewTwitchChat(j TwitchJoinHandler) *TwitchChat {
	c := &TwitchChat{
		messages:       make(map[string]chan *Message, 0),
		channels:       make([]string, 0),
		clients:        make(map[string]*twitchSocketClient, 0),
		channelClients: make(map[string]*twitchSocketClient, 0),
		joinHandler:    j,
		admins:         make(map[string]bool, len(GetConfig().Twitch.Admins)),
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

	return c
}

// Run ...
func (c *TwitchChat) Run() {
	go func() {
		if err := c.runCommandChannel(); err != nil {
			log.Panicf("error connecting to command channel %s", err)
		} else {
			log.Panicln("command channel closed unexppectedly")
		}
	}()

	for _, ch := range c.channels {
		if err := c.Join(ch, true); err != nil {
			log.Printf("error joining channel %s %s", ch, err)
		}
	}
}

func (c *TwitchChat) runCommandChannel() error {
	for {
		ch := GetConfig().Twitch.CommandChannel
		h, err := c.lookupHost(ch)
		if err != nil {
			return err
		}
		sc := newTwitchSocketClient(h)
		if err != nil {
			return err
		}
		mc, err := sc.Join(ch)
		if err != nil {
			return err
		}
		for {
			select {
			case m, ok := <-mc:
				if !ok {
					break
				}
				c.runCommand(sc, m)
			case <-sc.Evicted:
				break
			}
		}
	}
}

func (c *TwitchChat) runCommand(sc *twitchSocketClient, m *Message) {
	if _, ok := c.admins[m.Nick]; ok && m.Command == "MSG" {
		ch := GetConfig().Twitch.CommandChannel
		d := strings.Split(m.Data, " ")
		ld := strings.Split(strings.ToLower(m.Data), " ")

		if strings.EqualFold(d[0], "!join") {
			if err := c.Join(ld[1], false); err == nil {
				sc.Send("PRIVMSG #" + ch + " :Logging " + ld[1])
			} else {
				if err == ErrChannelNotValid {
					sc.Send("PRIVMSG #" + ch + " :Channel doesn't exist!")
				} else {
					sc.Send("PRIVMSG #" + ch + " :Already logging " + ld[1])
				}
			}
		} else if strings.EqualFold(d[0], "!leave") {
			if err := c.Leave(ld[1]); err == nil {
				sc.Send("PRIVMSG #" + ch + " :Leaving " + ld[1])
			} else {
				sc.Send("PRIVMSG #" + ch + " :Not logging " + ld[1])
			}
		} else if strings.EqualFold(d[0], "!channels") {
			sc.Send("PRIVMSG #" + ch + " :Logging " + strings.Join(c.channels, ", "))
		}
	}
}

// Join channel
func (c *TwitchChat) Join(ch string, init bool) error {
	h, err := c.lookupHost(ch)
	if err != nil {
		return err
	}
	c.Lock()
	if _, ok := c.channelClients[ch]; ok {
		c.Unlock()
		return ErrTwitchAlreadyInChannel
	}
	sc, ok := c.clients[h]
	if !ok {
		sc = newTwitchSocketClient(h)
		go c.runEvictHandler(sc)
		c.clients[h] = sc
	}
	c.channelClients[ch] = sc
	c.Unlock()

	m, err := sc.Join(ch)
	if err != nil {
		return err
	}
	go c.joinHandler(ch, m)

	if init {
		return nil
	}
	return c.saveChannels()
}

func (c *TwitchChat) runEvictHandler(sc *twitchSocketClient) {
	for {
		ch := <-sc.Evicted
		ch = strings.ToLower(ch)
		c.Lock()
		delete(c.channelClients, ch)
		if sc.Empty() {
			sc.Stop()
			delete(c.clients, sc.Host())
			c.Unlock()
			c.Join(ch, false)
			return
		}
		c.Unlock()
		c.Join(ch, false)
	}
}

// Leave channel
func (c *TwitchChat) Leave(ch string) error {
	ch = strings.ToLower(ch)
	c.Lock()
	sc, ok := c.channelClients[ch]
	c.Unlock()
	if !ok {
		return ErrTwitchNotInChannel
	}
	sc.Leave(ch)
	c.Lock()
	delete(c.channelClients, ch)
	c.Unlock()
	return c.saveChannels()
}

func (c *TwitchChat) lookupHost(ch string) (string, error) {
	ch = strings.ToLower(ch)
	u, err := url.Parse(TwitchChannelServerAPI)
	if err != nil {
		log.Panicf("error parsing twitch metadata endpoint url %s", err)
	}
	q := url.Values{}
	q.Add("channel", ch)
	u.RawQuery = q.Encode()
	res, err := http.Get(u.String())
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", ErrChannelMetadataNotAvailable
	}
	d := json.NewDecoder(res.Body)
	s := &struct {
		Cluster           string   `json:"cluster"`
		Servers           []string `json:"servers"`
		WebsocketsServers []string `json:"websockets_servers"`
	}{}
	if err := d.Decode(&s); err != nil {
		return "", err
	}
	return s.WebsocketsServers[0], nil
}

func (c *TwitchChat) saveChannels() error {
	c.Lock()
	defer c.Unlock()
	c.channels = []string{}
	for ch := range c.channelClients {
		c.channels = append(c.channels, ch)
	}
	f, err := os.Create(GetConfig().Twitch.ChannelListPath)
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

type twitchSocketClient struct {
	conn         *websocket.Conn
	messages     map[string]chan *Message
	sendLock     sync.Mutex
	connLock     sync.RWMutex
	messagesLock sync.RWMutex
	host         string
	stopped      bool
	retries      int
	Evicted      chan string
}

// NewTwitchChat new twitch chat client
func newTwitchSocketClient(host string) *twitchSocketClient {
	c := &twitchSocketClient{
		messages: make(map[string]chan *Message, 0),
		host:     host,
	}
	c.connect()
	go c.run()
	return c
}

func (c *twitchSocketClient) run() {
	messagePattern := regexp.MustCompile(`:(.+)\!.+tmi\.twitch\.tv PRIVMSG #([a-z0-9_-]+) :(.+)`)
	w := NewTimeWheel(TwitchReadTimeout, time.Second, c.evict)
	for {
		c.connLock.RLock()
		_, msg, err := c.conn.ReadMessage()
		if c.stopped == true {
			return
		}
		c.connLock.RUnlock()
		if err != nil {
			log.Printf("error reading from websocket %s", err)
			c.reconnect()
			continue
		}

		if strings.Index(string(msg), "PING") == 0 {
			c.Send(strings.Replace(string(msg), "PING", "PONG", -1))
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
				Nick:    v[1],
				Data:    data,
				Time:    time.Now().UTC(),
			}

			select {
			case mc <- m:
			default:
			}
		}
	}
}

func (c *twitchSocketClient) evict(ch string) {
	c.messagesLock.Lock()
	ch = strings.ToLower(ch)
	m, ok := c.messages[ch]
	if !ok {
		c.messagesLock.Unlock()
		return
	}
	log.Printf("channel evicted %s", ch)
	close(m)
	delete(c.messages, ch)
	c.messagesLock.Unlock()
	c.Send("PART #" + ch)
	c.Evicted <- ch
}

func (c *twitchSocketClient) connect() {
	var err error
	dialer := websocket.Dialer{HandshakeTimeout: SocketHandshakeTimeout}
	headers := http.Header{"Origin": []string{c.host}}
	c.connLock.Lock()
	c.conn, _, err = dialer.Dial(fmt.Sprintf("ws://%s/ws", c.host), headers)
	c.connLock.Unlock()
	if err != nil {
		log.Printf("error connecting to twitch ws %s", err)
		c.retries++
		if c.retries >= TwitchMaxConnectRetries {
			for ch := range c.messages {
				c.evict(ch)
			}
		}
		c.reconnect()
		return
	}
	c.retries = 0
	log.Printf("connected to %s", c.host)

	c.Send("PASS " + GetConfig().Twitch.OAuth)
	c.Send("NICK " + GetConfig().Twitch.Nick)

	for ch := range c.messages {
		c.Send("JOIN #" + ch)
	}
}

func (c *twitchSocketClient) reconnect() {
	if c.conn != nil {
		c.conn.Close()
	}
	time.Sleep(SocketReconnectDelay)
	c.connect()
}

func (c *twitchSocketClient) Stop() {
	c.connLock.Lock()
	c.stopped = true
	c.conn.Close()
	c.connLock.Unlock()
}

func (c *twitchSocketClient) Host() string {
	return c.host
}

func (c *twitchSocketClient) Empty() bool {
	c.messagesLock.RLock()
	defer c.messagesLock.RUnlock()
	return len(c.messages) == 0
}

func (c *twitchSocketClient) Join(ch string) (chan *Message, error) {
	ch = strings.ToLower(ch)
	c.messagesLock.Lock()
	m, ok := c.messages[ch]
	if ok {
		c.messagesLock.Unlock()
		return nil, ErrTwitchAlreadyInChannel
	}
	m = make(chan *Message, MessageBufferSize)
	c.messages[ch] = m
	c.messagesLock.Unlock()
	c.Send("JOIN #" + ch)
	return m, nil
}

// Leave channel
func (c *twitchSocketClient) Leave(ch string) error {
	ch = strings.ToLower(ch)
	c.messagesLock.Lock()
	_, ok := c.messages[ch]
	if !ok {
		c.messagesLock.Unlock()
		return ErrTwitchNotInChannel
	}
	delete(c.messages, ch)
	c.messagesLock.Unlock()
	defer c.Send("PART #" + ch)
	return nil
}

func (c *twitchSocketClient) Send(m string) {
	c.connLock.RLock()
	c.sendLock.Lock()
	err := c.conn.WriteMessage(websocket.TextMessage, []byte(m+"\r\n"))
	if err == nil {
		time.Sleep(SocketWriteDebounce)
	}
	c.sendLock.Unlock()
	c.connLock.RUnlock()
	if err != nil {
		log.Printf("error sending message %s", err)
		c.reconnect()
	}
}
