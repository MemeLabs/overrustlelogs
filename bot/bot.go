package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/slugalisk/overrustlelogs/common"
)

// log paths
const (
	destinyPath = "Destinygg chatlog"
	twitchPath  = "Destiny chatlog"
)

// errors
var (
	ErrIgnored     = errors.New("user ignored")
	ErrNukeTimeout = errors.New("overrustle nuked")
)

func init() {
	configPath := flag.String("config", "", "config path")
	flag.Parse()
	common.SetupConfig(*configPath)
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	c := common.NewDestinyChat()
	b := NewBot(c)
	go b.Run()
	go c.Run()

	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
	select {
	case <-sigint:
		b.Stop()
		log.Println("i love you guys, be careful")
		os.Exit(1)
	}
}

type command func(m *common.Message, r *bufio.Reader) (string, error)

// Bot commands
type Bot struct {
	c          *common.DestinyChat
	stop       chan bool
	start      time.Time
	timeoutEOL time.Time
	lastLine   string
	public     map[string]command
	private    map[string]command
	admins     map[string]struct{}
	ignore     map[string]struct{}
}

func NewBot(c *common.DestinyChat) *Bot {
	b := &Bot{
		c:      c,
		stop:   make(chan bool),
		start:  time.Now(),
		admins: make(map[string]struct{}, len(common.GetConfig().Bot.Admins)),
	}
	for _, admin := range common.GetConfig().Bot.Admins {
		b.admins[admin] = struct{}{}
	}
	b.public = map[string]command{
		"log":    b.handleDestinyLog,
		"tlog":   b.handleTwitchLog,
		"nuke":   b.handleNuke,
		"aegis":  b.handleAegis,
		"uptime": b.handleUptime,
		"bans":   b.handleBans,
		"subs":   b.handleSubs,
	}
	b.private = map[string]command{
		"p":        b.handlePremiumLog,
		"ignore":   b.handleIgnore,
		"unignore": b.handleUnignore,
	}
	b.ignore = make(map[string]struct{})
	d, err := ioutil.ReadFile(common.GetConfig().Bot.IgnoreListPath)
	if err != nil {
		log.Fatalf("unable to read ignore list %s", err)
	}
	ignore := []string{}
	if err := json.Unmarshal(d, &ignore); err != nil {
		log.Fatalf("unable to read ignore list %s", err)
	}
	for _, nick := range ignore {
		b.ignore[nick] = struct{}{}
	}
	return b
}

// Run start bot
func (b *Bot) Run() {
	for {
		select {
		case <-b.stop:
			return
		case m := <-b.c.Messages():
			if m.Command == "MSG" {
				if rs, err := b.runCommand(b.public, m); err == nil && rs != "" {
					if rs != b.lastLine {
						b.lastLine = rs
						if err := b.c.Write("MSG", rs); err != nil {
							log.Println(err)
						}
					}
				} else if err != nil {
					log.Println(err)
				}
			} else if m.Command == "PRIVMSG" {
				if rs, err := b.runCommand(b.private, m); err == nil && rs != "" {
					if err := b.c.WritePrivate("MSG", m.Nick, rs); err != nil {
						log.Println(err)
					}
				} else if err != nil {
					log.Println(err)
				}
			}
		}
	}
}

// Stop bot
func (b *Bot) Stop() {
	b.stop <- true
	ignore := []string{}
	for nick := range b.ignore {
		ignore = append(ignore, nick)
	}
	data, err := json.Marshal(ignore)
	if err != nil {
		log.Fatalf("unable to write ignore list %s", err)
	}
	if err := ioutil.WriteFile(common.GetConfig().Bot.IgnoreListPath, data, 0644); err != nil {
		log.Fatalf("unable to write ignore list %s", err)
	}
}

func (b *Bot) runCommand(commands map[string]command, m *common.Message) (string, error) {
	if string(m.Data[0]) == "!" {
		if b.isIgnored(m.Nick) {
			return "", ErrIgnored
		}
		r := bufio.NewReader(bytes.NewReader([]byte(m.Data[1:])))
		c, err := r.ReadString(' ')
		if err != nil && err != io.EOF {
			return "", err
		}
		if err != io.EOF {
			c = c[:len(c)-1]
		}
		if cmd, ok := commands[c]; ok {
			return cmd(m, r)
		}
	}
	return "", nil
}

func (b *Bot) isAdmin(nick string) bool {
	_, ok := b.admins[nick]
	return ok
}

func (b *Bot) isIgnored(nick string) bool {
	_, ok := b.ignore[strings.ToLower(nick)]
	return ok
}

func (b *Bot) handlePremiumLog(m *common.Message, r *bufio.Reader) (string, error) {
	if !time.Now().After(b.timeoutEOL) {
		return "", ErrNukeTimeout
	}
	return common.GetConfig().LogHost + "/" + destinyPath + "/premium/" + m.Nick + "/" + time.Now().Format("January 2006") + ".txt", nil
}

func (b *Bot) handleIgnore(m *common.Message, r *bufio.Reader) (string, error) {
	if b.isAdmin(m.Nick) {
		nick, err := ioutil.ReadAll(r)
		if err != nil {
			return "", err
		}
		b.ignore[strings.ToLower(string(nick))] = struct{}{}
	}
	return "", nil
}

func (b *Bot) handleUnignore(m *common.Message, r *bufio.Reader) (string, error) {
	if b.isAdmin(m.Nick) {
		nick, err := ioutil.ReadAll(r)
		if err != nil {
			return "", err
		}
		delete(b.ignore, strings.ToLower(string(nick)))
	}
	return "", nil
}

func (b *Bot) handleDestinyLog(m *common.Message, r *bufio.Reader) (string, error) {
	return b.handleLog(destinyPath, r)
}

func (b *Bot) handleTwitchLog(m *common.Message, r *bufio.Reader) (string, error) {
	return b.handleLog(twitchPath, r)
}

func (b *Bot) handleLog(path string, r *bufio.Reader) (string, error) {
	if !time.Now().After(b.timeoutEOL) {
		return "", ErrNukeTimeout
	}
	nick, err := ioutil.ReadAll(r)
	if err != nil {
		return "", err
	}
	s, err := common.NewNickSearch(common.GetConfig().LogPath+"/"+path, string(nick))
	if err != nil {
		return "", err
	}
	rs, err := s.Next()
	if err != nil {
		return "", err
	}
	return rs.Month() + " logs. " + common.GetConfig().LogHost + "/" + path + "/" + rs.Month() + "/userlogs/" + rs.Nick() + ".txt", nil
}

func (b *Bot) handleNuke(m *common.Message, r *bufio.Reader) (string, error) {
	if b.isAdmin(m.Nick) {
		word, err := ioutil.ReadAll(r)
		if err != nil {
			return "", err
		}
		if bytes.Contains([]byte("overrustle"), bytes.ToLower(word)) {
			b.timeoutEOL = time.Now().Add(30 * time.Minute)
		}
	}
	return "", nil
}

func (b *Bot) handleAegis(m *common.Message, r *bufio.Reader) (string, error) {
	if b.isAdmin(m.Nick) {
		b.timeoutEOL = time.Now()
	}
	return "", nil
}

func (b *Bot) handleBans(m *common.Message, r *bufio.Reader) (string, error) {
	return common.GetConfig().LogHost + "/" + destinyPath + "/" + time.Now().Format("January 2006") + "/bans.txt", nil
}

func (b *Bot) handleSubs(m *common.Message, r *bufio.Reader) (string, error) {
	return common.GetConfig().LogHost + "/" + destinyPath + "/" + time.Now().Format("January 2006") + "/subscribers.txt", nil
}

func (b *Bot) handleUptime(m *common.Message, r *bufio.Reader) (string, error) {
	return time.Since(b.start).String(), nil
}
