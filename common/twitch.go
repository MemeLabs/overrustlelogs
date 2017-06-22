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

// Twitch twitch chat client
type Twitch struct {
	connLock       sync.Mutex
	sendLock       sync.Mutex
	conn           *websocket.Conn
	ChLock         sync.Mutex
	channels       []string
	messages       chan *Message
	MessagePattern *regexp.Regexp
	SubPattern     *regexp.Regexp
	quit           chan struct{}
}

// NewTwitch new twitch chat client
func NewTwitch() *Twitch {
	return &Twitch{
		channels: make([]string, 0),
		messages: make(chan *Message, MessageBufferSize),
		// > @badges=global_mod/1,turbo/1;color=#0D4200;display-name=dallas;emotes=25:0-4,12-16/1902:6-10;mod=0;room-id=1337;
		//subscriber=0;turbo=1;user-id=1337;user-type=global_mod :ronni!ronni@ronni.tmi.twitch.tv PRIVMSG #dallas :Kappa Keepo Kappa
		MessagePattern: regexp.MustCompile(`user-type=.+:([a-z0-9_-]+)\!.+\.tmi\.twitch\.tv PRIVMSG #([a-z0-9_-]+) :(.+)`),
		// > @badges=staff/1,broadcaster/1,turbo/1;color=#008000;display-name=ronni;emotes=;mod=0;msg-id=resub;msg-param-months=6;
		// msg-param-sub-plan=Prime;msg-param-sub-plan-name=Prime;room-id=1337;subscriber=1;system-msg=ronni\shas\ssubscribed\sfor\s6\smonths!;
		// login=ronni;turbo=1;user-id=1337;user-type=staff :tmi.twitch.tv USERNOTICE #dallas :Great stream -- keep it up!
		SubPattern: regexp.MustCompile(`system-msg=(.+);tmi-sent-ts.+ \:tmi\.twitch\.tv USERNOTICE #([a-z0-9_-]+)( :.+)?`),
		quit:       make(chan struct{}, 2),
	}
}

func (c *Twitch) connect() {
	conf := GetConfig()
	dialer := websocket.Dialer{HandshakeTimeout: HandshakeTimeout}
	headers := http.Header{"Origin": []string{conf.Twitch.OriginURL}}

	var err error
	c.connLock.Lock()
	c.conn, _, err = dialer.Dial(GetConfig().Twitch.SocketURL, headers)
	c.connLock.Unlock()
	if err != nil {
		log.Printf("error connecting to twitch ws %s", err)
		c.reconnect()
		return
	}

	if conf.Twitch.OAuth == "" || conf.Twitch.Nick == "" {
		log.Println("missing OAuth or Nick, using justinfan659 as login data")
		conf.Twitch.OAuth = "justinfan659"
		conf.Twitch.Nick = "justinfan659"
	}

	c.send("PASS " + conf.Twitch.OAuth)
	c.send("NICK " + conf.Twitch.Nick)
	c.send("CAP REQ :twitch.tv/tags")
	c.send("CAP REQ :twitch.tv/commands")

	for _, ch := range c.channels {
		select {
		case <-c.quit:
			return
		default:
		}
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
		for {
			select {
			case <-c.quit:
				close(c.messages)
				return
			default:
			}
			c.connLock.Lock()
			err := c.conn.SetReadDeadline(time.Now().Add(SocketReadTimeout))
			c.connLock.Unlock()
			if err != nil {
				log.Printf("error setting the ReadDeadline: %v", err)
				c.reconnect()
				continue
			}

			c.connLock.Lock()
			_, msg, err := c.conn.ReadMessage()
			c.connLock.Unlock()
			if err != nil {
				log.Printf("error reading message: %v", err)
				c.reconnect()
				continue
			}

			if strings.Index(string(msg), "PING") == 0 {
				err := c.send(strings.Replace(string(msg), "PING", "PONG", -1))
				if err != nil {
					log.Printf("error sending PONG: %v", err)
					c.reconnect()
				}
				continue
			}

			s := c.SubPattern.FindAllStringSubmatch(string(msg), -1)
			for _, v := range s {
				data := strings.Replace(v[1], "\\s", " ", -1)
				if v[3] != "" {
					data += " [SubMessage]: " + v[3][2:]
				}
				m := &Message{
					Type:    "MSG",
					Channel: v[2],
					Nick:    "twitchnotify",
					Data:    data,
					Time:    time.Now().UTC(),
				}

				select {
				case c.messages <- m:
				default:
					log.Println("error messages channel full :(")
				}
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
		return errors.New("already in channel")
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
	return errors.New("not in channel")
}

func (c *Twitch) rejoinHandler() {
	const interval = 1 * time.Hour
	ticker := time.NewTicker(interval)
	for {
		select {
		case <-c.quit:
			return
		case <-ticker.C:
			c.ChLock.Lock()
			for _, ch := range c.channels {
				ch = strings.ToLower(ch)
				log.Printf("rejoining %s\n", ch)
				if err := c.send("JOIN #" + ch); err != nil {
					log.Println(err)
					continue
				}
			}
			c.ChLock.Unlock()
		}
	}
}

// Stop stops the chats
func (c *Twitch) Stop(wg *sync.WaitGroup) {
	close(c.quit)
	c.connLock.Lock()
	if c.conn != nil {
		c.conn.Close()
	}
	c.connLock.Unlock()
	wg.Done()
}

func inSlice(s []string, v string) bool {
	for _, sv := range s {
		if strings.EqualFold(sv, v) {
			return true
		}
	}
	return false
}
