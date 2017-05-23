package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/slugalisk/overrustlelogs/common"
)

var (
	ErrNotInChannel     = errors.New("not in channel")
	ErrAlreadyInChannel = errors.New("already in channel")
	ErrChannelNotValid  = errors.New("channel not valid")
)

// TwitchLogger ...
type TwitchLogger struct {
	chatLock sync.RWMutex
	chats    map[int]*common.Twitch

	chLock   sync.RWMutex
	channels []string

	logHandler     func(m <-chan *common.Message)
	admins         map[string]struct{}
	commandChannel string
}

// NewTwitchLogger ...
func NewTwitchLogger(f func(m <-chan *common.Message)) *TwitchLogger {
	t := &TwitchLogger{
		chats:      make(map[int]*common.Twitch),
		admins:     make(map[string]struct{}),
		logHandler: f,
	}

	admins := common.GetConfig().Twitch.Admins
	for _, a := range admins {
		t.admins[a] = struct{}{}
	}

	d, err := ioutil.ReadFile(common.GetConfig().Twitch.ChannelListPath)
	if err != nil {
		log.Fatalf("unable to read channels %s", err)
	}

	if err := json.Unmarshal(d, &t.channels); err != nil {
		log.Fatalf("unable to read channels %s", err)
	}

	t.commandChannel = common.GetConfig().Twitch.CommandChannel

	return t
}

// Start ...
func (t *TwitchLogger) Start() {
	t.join(common.GetConfig().Twitch.CommandChannel, false)
	var c int
	for _, channel := range t.channels {
		err := t.join(channel, false)
		if err != nil {
			log.Printf("failed to join %s err: %s", channel, err.Error())
			continue
		}
		c++
	}
	log.Println("joined", c, "chats, wew lad :^)")
}

// Stop ...
func (t *TwitchLogger) Stop() {
	t.saveChannels()
	t.chatLock.Lock()
	for id, c := range t.chats {
		log.Printf("stopping chat: %d\n", id)
		c.Stop()
	}
	t.chatLock.Unlock()
}

func (t *TwitchLogger) join(ch string, init bool) error {
	if init {
		if !channelExists(ch) {
			return ErrChannelNotValid
		}
		if inSlice(t.channels, ch) {
			return ErrAlreadyInChannel
		}
		t.addChannel(ch)
		if err := t.saveChannels(); err != nil {
			log.Println(err)
		}
	}
	c := t.getChatToJoin()
	for try := 1; try <= 3; try++ {
		err := c.Join(ch)
		if err == nil {
			log.Printf("joining %s", ch)
			return nil
		}
		if err != nil && try == 3 {
			return fmt.Errorf("failed to join %s :( %v", ch, err)
		}
		log.Println("retrying to join", ch)
	}
	return nil
}

func (t *TwitchLogger) leave(ch string) error {
	t.chLock.Lock()
	if !inSlice(t.channels, ch) {
		t.chLock.Unlock()
		return ErrNotInChannel
	}
	t.chLock.Unlock()

	t.chatLock.Lock()
	for i, c := range t.chats {
		if inSlice(c.Channels(), ch) {
			c.Leave(ch)
			log.Printf("found channel in chat %d", i)
			break
		}
	}
	t.chatLock.Unlock()
	t.removeChannel(ch)
	err := t.saveChannels()
	if err != nil {
		log.Println(err)
	}
	log.Println("left", ch)
	return err
}

func (t *TwitchLogger) getChatToJoin() *common.Twitch {
	t.chatLock.Lock()
	defer t.chatLock.Unlock()
	for _, c := range t.chats {
		c.ChLock.Lock()
		if len(c.Channels()) < common.MaxChannelsPerChat {
			c.ChLock.Unlock()
			return c
		}
		c.ChLock.Unlock()
	}
	c := common.NewTwitch()
	c.Run()
	i := len(t.chats) + 1
	t.chats[i] = c
	go t.msgHandler(i, c.Messages())
	return c
}

func (t *TwitchLogger) msgHandler(i int, ch <-chan *common.Message) {
	logCh := make(chan *common.Message, common.MessageBufferSize)
	defer close(logCh)
	go t.logHandler(logCh)
	for m := range ch {
		if t.commandChannel == m.Channel {
			go t.runCommand(i, m)
		}
		select {
		case logCh <- m:
		default:
			log.Println("error buffer is full")
		}
	}
}

func (t *TwitchLogger) runCommand(i int, m *common.Message) {
	c, ok := t.chats[i]
	if !ok {
		return
	}
	if _, ok := t.admins[m.Nick]; !ok && m.Type != "MSG" {
		return
	}
	parts := strings.Split(strings.ToLower(m.Data), " ")
	switch parts[0] {
	case "!join":
		err := t.join(parts[1], true)
		if err != nil {
			c.Message(m.Channel, err.Error())
			return
		}
		c.Message(m.Channel, fmt.Sprintf("Logging %s", parts[1]))
	case "!leave":
		err := t.leave(parts[1])
		if err != nil {
			c.Message(m.Channel, fmt.Sprintf("Not logging %s", parts[1]))
			return
		}
		c.Message(m.Channel, fmt.Sprintf("Leaving %s", parts[1]))
	}
}

func (t *TwitchLogger) addChannel(ch string) {
	t.chLock.Lock()
	t.channels = append(t.channels, ch)
	t.chLock.Unlock()
}

func (t *TwitchLogger) removeChannel(ch string) error {
	t.chLock.Lock()
	defer t.chLock.Unlock()
	sort.Strings(t.channels)
	i := sort.SearchStrings(t.channels, ch)
	if i < len(t.channels) && t.channels[i] == ch {
		t.channels = append(t.channels[:i], t.channels[i+1:]...)
		return nil
	}
	return errors.New("didn't find " + ch + " in the channels list")
}

func (t *TwitchLogger) saveChannels() error {
	f, err := os.Create(common.GetConfig().Twitch.ChannelListPath)
	if err != nil {
		log.Printf("error saving channel list %s", err)
		return err
	}
	defer f.Close()
	sort.Strings(t.channels)
	data, err := json.Marshal(t.channels)
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

// channelExists
func channelExists(ch string) bool {
	req, err := http.NewRequest("GET", "https://api.twitch.tv/kraken/users/"+strings.ToLower(ch), nil)
	if err != nil {
		return false
	}
	req.Header.Add("Client-ID", common.GetConfig().Twitch.ClientID)
	client := http.Client{
		Timeout: 10 * time.Second,
	}
	res, err := client.Do(req)
	if err != nil {
		log.Println(err)
		return false
	}
	res.Body.Close()
	return res.StatusCode == http.StatusOK
}

func inSlice(slice []string, s string) bool {
	for _, v := range slice {
		if strings.EqualFold(s, v) {
			return true
		}
	}
	return false
}
