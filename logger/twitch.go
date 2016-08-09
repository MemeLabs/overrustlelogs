package main

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
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/slugalisk/overrustlelogs/chat"
	"github.com/slugalisk/overrustlelogs/common"
)

var (
	debug               bool
	errNotInChannel     = errors.New("not in channel")
	errAlreadyInChannel = errors.New("already in channel")
	errChannelNotValid  = errors.New("channel not validq")
)

// Message ...
type Message struct {
	common.Message
}

// TwitchLogger ...
type TwitchLogger struct {
	chatLock       sync.RWMutex
	chats          map[int]*chat.Twitch
	admins         map[string]struct{}
	chLock         sync.RWMutex
	channels       []string
	logHandler     func(m <-chan *common.Message)
	commandChannel string
}

// NewTwitchLogger ...
func NewTwitchLogger(f func(m <-chan *common.Message)) *TwitchLogger {
	t := &TwitchLogger{
		chats:      make(map[int]*chat.Twitch, 0),
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
	var count int
	var chatID = 1
	for _, channel := range t.channels {
		if count == common.MaxChannelsPerChat {
			log.Printf("chat %d reached %d joined channels\n", chatID, common.MaxChannelsPerChat)
			chatID++
			count = 0
		}

		if _, ok := t.chats[chatID]; !ok {
			log.Println("starting twitch chat client", chatID)
			_, err := t.startNewChat(chatID)
			if err != nil {
				log.Println("REEEEEEEEEEEEEEEEEE")
			}
		}

		err := t.join(channel, false)
		if err != nil {
			log.Println("failed to join", channel)
			continue
		}
		count++
	}
}

// Stop ...
func (t *TwitchLogger) Stop() {
	t.chatLock.Lock()
	defer t.chatLock.Unlock()
	for id, c := range t.chats {
		log.Printf("stopping chat: %d\n", id)
		c.Stop()
	}
}

func (t *TwitchLogger) getChatToJoin() (int, *chat.Twitch) {
	t.chLock.Lock()
	defer t.chLock.Unlock()
	for id, c := range t.chats {
		if len(c.Channels()) < common.MaxChannelsPerChat {
			return id, c
		}
	}
	id := len(t.chats) + 1
	c, _ := t.startNewChat(id)
	return id, c
}

func (t *TwitchLogger) join(ch string, init bool) error {
	if init {
		if inSlice(t.channels, ch) {
			return errAlreadyInChannel
		}
		if !channelExists(ch) {
			return errChannelNotValid
		}
		t.addChannel(ch)
		err := t.saveChannels()
		if err != nil {
			log.Println(err)
		}
	}
	id, c := t.getChatToJoin()
	for try := 1; try <= 3; try++ {
		err := c.Join(ch)
		if err == nil {
			log.Printf("joining %s on chat %d.", ch, id)
			return nil
		}
		if err != nil && try == 3 {
			return errors.New("failed to join " + ch + " :(")
		}
		log.Println("retrying to join", ch)
	}
	return nil
}

func (t *TwitchLogger) leave(ch string) error {
	if !inSlice(t.channels, ch) {
		return errNotInChannel
	}
	for i, c := range t.chats {
		if inSlice(c.Channels(), ch) {
			c.Leave(ch)
			log.Printf("found channel in chat %d", i)
			break
		}
	}
	t.removeChannel(ch)
	err := t.saveChannels()
	if err != nil {
		log.Println(err)
	}
	log.Println("left", ch)
	return err
}

func (t *TwitchLogger) startNewChat(id int) (*chat.Twitch, error) {
	newChat := chat.NewTwitch()
	newChat.Debug(debug)
	go newChat.Run()
	go t.msgHandler(id, newChat.Messages())
	t.chatLock.Lock()
	if _, ok := t.chats[id]; ok {
		newChat.Stop()
		return nil, fmt.Errorf("a chat exists already with the id: %d.\n", id)
	}
	t.chats[id] = newChat
	t.chatLock.Unlock()
	time.Sleep(5 * time.Second)
	return newChat, nil
}

func (t *TwitchLogger) msgHandler(chatID int, ch <-chan *common.Message) {
	logCh := make(chan *common.Message, common.MessageBufferSize)
	defer close(logCh)
	go t.logHandler(logCh)
	for m := range ch {
		if t.commandChannel == m.Channel {
			go t.runCommand(chatID, m)
		}
		select {
		case logCh <- m:
		default:
		}
	}
}

func (t *TwitchLogger) runCommand(chatID int, m *common.Message) {
	c := t.chats[chatID]
	if _, ok := t.admins[m.Nick]; !ok && m.Command != "MSG" {
		return
	}
	ld := strings.Split(strings.ToLower(m.Data), " ")
	switch ld[0] {
	case "!join":
		err := t.join(ld[1], true)
		switch err {
		case nil:
			c.Message(m.Channel, fmt.Sprintf("Logging %s", ld[1]))
		case errChannelNotValid:
			c.Message(m.Channel, "Channel doesn't exist!")
		case errAlreadyInChannel:
			c.Message(m.Channel, fmt.Sprintf("Already logging %s", ld[1]))
		default:
		}
	case "!leave":
		err := t.leave(ld[1])
		if err != nil {
			c.Message(m.Channel, fmt.Sprintf("Not logging %s", ld[1]))
			return
		}
		c.Message(m.Channel, fmt.Sprintf("Leaving %s", ld[1]))
	}
}

func (t *TwitchLogger) addChannel(ch string) {
	t.chLock.Lock()
	defer t.chLock.Unlock()
	t.channels = append(t.channels, ch)
	log.Println("added", ch, "to list")
}

func (t *TwitchLogger) removeChannel(ch string) error {
	t.chLock.Lock()
	defer t.chLock.Unlock()
	for i, channel := range t.channels {
		if strings.EqualFold(channel, ch) {
			t.channels = append(t.channels[:i], t.channels[i+1:]...)
			return nil
		}
	}
	return errors.New("didn't find " + ch)
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
	u, err := url.Parse("https://api.twitch.tv/kraken/users/" + ch)
	if err != nil {
		log.Panicf("error parsing twitch metadata endpoint url %s", err)
	}
	req := &http.Request{
		Header: http.Header{
			"Client-ID": []string{common.GetConfig().Twitch.ClientID},
		},
		URL: u,
	}
	client := http.Client{}
	res, _ := client.Do(req)
	res.Body.Close()
	return res.StatusCode == http.StatusOK
}

func inSlice(s []string, v string) bool {
	for _, sv := range s {
		if strings.EqualFold(v, sv) {
			return true
		}
	}
	return false
}
