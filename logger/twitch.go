package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/MemeLabs/overrustlelogs/common"
)

// Paths
const (
	ChannelListPath = "/logger/channels.json"
)

// TwitchHub ...
type TwitchHub struct {
	chatLock       sync.RWMutex
	chats          []*common.Twitch
	chLock         sync.RWMutex
	channels       []string
	logHandler     func(m <-chan *common.Message)
	admins         map[string]struct{}
	commandChannel string
	quit           chan struct{}
}

// NewTwitchLogger ...
func NewTwitchLogger(f func(m <-chan *common.Message)) *TwitchHub {
	t := &TwitchHub{
		logHandler:     f,
		admins:         make(map[string]struct{}),
		commandChannel: common.GetConfig().Twitch.CommandChannel,
		quit:           make(chan struct{}, 1),
	}

	admins := common.GetConfig().Twitch.Admins
	for _, a := range admins {
		t.admins[a] = struct{}{}
	}

	d, err := ioutil.ReadFile("/logger/channels.json")
	if err != nil {
		log.Fatalf("unable to read channels %s", err)
	}
	if err := json.Unmarshal(d, &t.channels); err != nil {
		log.Fatalf("unable to read channels %s", err)
	}
	return t
}

// Start ...
func (t *TwitchHub) Start() {
	var c int
	for _, channel := range t.channels {
		select {
		case <-t.quit:
			return
		default:
		}
		err := t.join(channel, false)
		if err != nil {
			log.Printf("%v", err)
			continue
		}
		c++
	}
	log.Printf("joined %d chats, wew lad :^)\n", c)
}

// Stop ...
func (t *TwitchHub) Stop() {
	close(t.quit)
	var wg sync.WaitGroup

	t.chatLock.Lock()
	wg.Add(len(t.chats))
	for i, c := range t.chats {
		log.Printf("stopping chat: %d\n", i)
		go c.Stop(&wg)
	}
	t.chatLock.Unlock()
	wg.Wait()
}

func (t *TwitchHub) runCommand(c *common.Twitch, m *common.Message) {
	if _, ok := t.admins[m.Nick]; !ok || m.Type != "MSG" {
		return
	}

	parts := strings.Split(strings.ToLower(m.Data), " ")
	switch parts[0] {
	case "!join":
		if err := t.join(strings.TrimSpace(parts[1]), true); err != nil {
			log.Println(err)
			log.Println(c.Message(m.Channel, err.Error()))
			return
		}
		c.Message(m.Channel, fmt.Sprintf("Logging %s", strings.TrimSpace(parts[1])))
	case "!leave":
		if err := t.leave(parts[1]); err != nil {
			log.Println(err)
			c.Message(m.Channel, fmt.Sprintf("Not logging %s", strings.TrimSpace(parts[1])))
			return
		}
		c.Message(m.Channel, fmt.Sprintf("Leaving %s", parts[1]))
	}
}

func (t *TwitchHub) join(ch string, init bool) error {
	if inSlice(t.channels, ch) && init {
		return fmt.Errorf("already logging %s", ch)
	}

	exists, _ := channelExists(ch)
	if !exists && init {
		return fmt.Errorf("%s doesn't exist my dude", ch)
	}

	if init {
		t.addChannel(ch)
		go t.saveChannels()
	}

	t.chatLock.Lock()
	var chat *common.Twitch
	for _, c := range t.chats {
		if len(c.Channels()) >= common.MaxChannelsPerChat {
			continue
		}
		chat = c
	}
	if chat == nil {
		chat = common.NewTwitch()
		chat.Run()
		t.chats = append(t.chats, chat)
		go t.msgHandler(chat)
	}
	t.chatLock.Unlock()
	if err := chat.Join(ch); err != nil {
		return fmt.Errorf("failed to join %s: %v", ch, err)
	}
	log.Println("joined", ch)
	return nil
}

func (t *TwitchHub) msgHandler(c *common.Twitch) {
	messages := make(chan *common.Message, common.MessageBufferSize)
	go t.logHandler(messages)
	for {
		select {
		case <-t.quit:
			close(messages)
			return
		case m := <-c.Messages():
			messages <- m
			if t.commandChannel == m.Channel {
				go t.runCommand(c, m)
			}
		}
	}
}

func (t *TwitchHub) leave(ch string) error {
	if err := t.removeChannel(ch); err != nil {
		return err
	}
	if err := t.saveChannels(); err != nil {
		return err
	}

	t.chatLock.Lock()
	defer t.chatLock.Unlock()
	for _, c := range t.chats {
		if !inSlice(c.Channels(), ch) {
			continue
		}
		if err := c.Leave(ch); err != nil {
			return fmt.Errorf("error leaving %s: %v", ch, err)
		}

		log.Println("leaving", ch)
		return nil
	}
	return fmt.Errorf("%s not found", ch)
}

func (t *TwitchHub) addChannel(ch string) {
	t.chLock.Lock()
	t.channels = append(t.channels, ch)
	t.chLock.Unlock()
}

func (t *TwitchHub) removeChannel(ch string) error {
	t.chLock.Lock()
	defer t.chLock.Unlock()
	sort.Strings(t.channels)
	for i, c := range t.channels {
		if strings.EqualFold(ch, c) {
			t.channels = append(t.channels[:i], t.channels[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("didn't find %s in the channels list", ch)
}

func (t *TwitchHub) saveChannels() error {
	f, err := os.Create("/logger/channels.json")
	if err != nil {
		log.Printf("error saving channel list %s", err)
		return err
	}
	defer f.Close()

	t.chLock.RLock()
	sort.Strings(t.channels)
	data, err := json.Marshal(t.channels)
	if err != nil {
		log.Printf("error saving channel list %s", err)
		return err
	}
	t.chLock.RUnlock()

	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", "\t"); err != nil {
		log.Printf("error saving channel list %s", err)
		return err
	}
	f.Write(buf.Bytes())
	return nil
}

var client = http.Client{
	Timeout: 5 * time.Second,
}

type usersResponse struct {
	Total int `json:"_total"`
	Users []struct {
		ID          string    `json:"_id"`
		Bio         string    `json:"bio"`
		CreatedAt   time.Time `json:"created_at"`
		DisplayName string    `json:"display_name"`
		Logo        string    `json:"logo"`
		Name        string    `json:"name"`
		Type        string    `json:"type"`
		UpdatedAt   time.Time `json:"updated_at"`
	} `json:"users"`
}

// channelExists
func channelExists(ch string) (bool, string) {
	req, err := http.NewRequest("GET", "https://api.twitch.tv/kraken/users?login="+strings.ToLower(ch), nil)
	if err != nil {
		return false, ""
	}
	req.Header.Add("Accept", "application/vnd.twitchtv.v5+json")
	req.Header.Add("Client-ID", common.GetConfig().Twitch.ClientID)
	res, err := client.Do(req)
	if err != nil {
		return false, ""
	}

	if res.StatusCode != http.StatusOK {
		return false, ""
	}

	var us usersResponse
	err = json.NewDecoder(res.Body).Decode(&us)
	if us.Total == 0 {
		return false, ""
	}

	return true, us.Users[0].ID
}

func inSlice(slice []string, s string) bool {
	for _, v := range slice {
		if strings.EqualFold(s, v) {
			return true
		}
	}
	return false
}
