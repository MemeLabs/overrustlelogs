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
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/slugalisk/overrustlelogs/common"
)

// log paths
const (
	destinyPath         = "Destinygg chatlog"
	twitchPath          = "Destiny chatlog"
	defaultNukeDuration = 10 * time.Minute
	cooldownDuration    = 10 * time.Second
)

// errors
var (
	ErrIgnored     = errors.New("user ignored")
	ErrNukeTimeout = errors.New("overrustle nuked")
	ErrInvalidNick = errors.New("invalid nick")
)

var validNick = regexp.MustCompile("^[a-zA-Z0-9_]+$")

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
		os.Exit(0)
	}
}

type command func(m *common.Message, r *bufio.Reader) (string, error)

// Bot commands
type Bot struct {
	c           *common.DestinyChat
	stop        chan bool
	start       time.Time
	nukeEOL     time.Time
	nukeText    []byte
	lastLine    string
	cooldownEOL time.Time
	public      map[string]command
	private     map[string]command
	admins      map[string]struct{}
	ignore      map[string]struct{}
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
		"logs":   b.handleDestinyLogs,
		"log":    b.handleDestinyLogs,
		"tlogs":  b.handleTwitchLogs,
		"tlog":   b.handleTwitchLogs,
		"nuke":   b.handleSimpleNuke,
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
	if d, err := ioutil.ReadFile(common.GetConfig().Bot.IgnoreListPath); err == nil {
		ignore := []string{}
		if err := json.Unmarshal(d, &ignore); err == nil {
			for _, nick := range ignore {
				b.addIgnore(nick)
			}
		}
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
					if b.isNuked(rs) {
						b.addIgnore(m.Nick)
					} else if rs != b.lastLine && (b.isAdmin(m.Nick) || time.Now().After(b.cooldownEOL)) {
						b.lastLine = rs
						b.cooldownEOL = time.Now().Add(cooldownDuration)
						if err := b.c.Write("MSG", rs); err != nil {
							log.Println(err)
						}
					}
				} else if err != nil {
					log.Println(err)
				}
			} else if m.Command == "PRIVMSG" {
				if rs, err := b.runCommand(b.private, m); err == nil && rs != "" {
					if err := b.c.WritePrivate("PRIVMSG", m.Nick, rs); err != nil {
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
	data, _ := json.Marshal(ignore)
	if err := ioutil.WriteFile(common.GetConfig().Bot.IgnoreListPath, data, 0644); err != nil {
		log.Fatalf("unable to write ignore list %s", err)
	}
}

func (b *Bot) runCommand(commands map[string]command, m *common.Message) (string, error) {
	if m.Data[0] == '!' {
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
		} else if len(c) >= 4 && strings.EqualFold(c[0:4], "nuke") {
			return b.handleCustomNuke(m, c[4:], r)
		}
	}
	return "", nil
}

func (b *Bot) isNuked(text string) bool {
	return b.nukeEOL.After(time.Now()) && bytes.Contains(bytes.ToLower([]byte(text)), b.nukeText)
}

func (b *Bot) isAdmin(nick string) bool {
	_, ok := b.admins[nick]
	return ok
}

func (b *Bot) isIgnored(nick string) bool {
	_, ok := b.ignore[strings.ToLower(nick)]
	return ok
}

func (b *Bot) addIgnore(nick string) {
	b.ignore[strings.ToLower(nick)] = struct{}{}
}

func (b *Bot) removeIgnore(nick string) {
	delete(b.ignore, strings.ToLower(string(nick)))
}

func (b *Bot) toURL(path string) string {
	var u, err = url.Parse(common.GetConfig().LogHost)
	if err != nil {
		log.Fatalf("error parsing configured log host %s", err)
	}
	u.Path = path
	return u.String()
}

func (b *Bot) handlePremiumLog(m *common.Message, r *bufio.Reader) (string, error) {
	return b.toURL("/" + destinyPath + "/premium/" + m.Nick + "/" + time.Now().UTC().Format("January 2006") + ".txt"), nil
}

func (b *Bot) handleIgnore(m *common.Message, r *bufio.Reader) (string, error) {
	if b.isAdmin(m.Nick) {
		nick, err := ioutil.ReadAll(r)
		if err != nil || !validNick.Match(nick) {
			return "Invalid nick", err
		}
		b.addIgnore(string(nick))
	}
	return "", nil
}

func (b *Bot) handleUnignore(m *common.Message, r *bufio.Reader) (string, error) {
	if b.isAdmin(m.Nick) {
		nick, err := ioutil.ReadAll(r)
		if err != nil || !validNick.Match(nick) {
			return "Invalid nick", err
		}
		b.removeIgnore(string(nick))
	}
	return "", nil
}

func (b *Bot) handleDestinyLogs(m *common.Message, r *bufio.Reader) (string, error) {
	return b.handleLog(destinyPath, r)
}

func (b *Bot) handleTwitchLogs(m *common.Message, r *bufio.Reader) (string, error) {
	return b.handleLog(twitchPath, r)
}

func (b *Bot) handleLog(path string, r *bufio.Reader) (string, error) {
	nick, err := r.ReadString(' ')
	log.Println(nick, ":nick")
	nick = strings.TrimSpace(nick)
	if (err != nil && err != io.EOF) || len(nick) < 1 {
		log.Println("err", err)
		return b.toURL("/" + path + "/" + time.Now().UTC().Format("January 2006") + "/"), nil
	}
	if !validNick.Match([]byte(nick)) {
		return "", ErrInvalidNick
		log.Println(ErrInvalidNick.Error())
	}
	s, err := common.NewNickSearch(common.GetConfig().LogPath+"/"+path, string(nick))
	if err != nil {
		return "", err
		log.Println("err:", err)
	}
	rs, err := s.Next()
	if err != nil {
		return "No logs found for that user.", nil
	}
	return rs.Month() + " logs. " + b.toURL("/"+path+"/"+rs.Month()+"/userlogs/"+rs.Nick()+".txt"), nil
}

func (b *Bot) handleSimpleNuke(m *common.Message, r *bufio.Reader) (string, error) {
	return b.handleNuke(m, defaultNukeDuration, r)
}

func (b *Bot) handleCustomNuke(m *common.Message, d string, r *bufio.Reader) (string, error) {
	if s, err := strconv.ParseUint(d, 10, 64); err == nil {
		return b.handleNuke(m, time.Duration(s)*time.Second, r)
	}
	return "", nil
}

func (b *Bot) handleNuke(m *common.Message, d time.Duration, r *bufio.Reader) (string, error) {
	if b.isAdmin(m.Nick) {
		text, err := ioutil.ReadAll(r)
		if err != nil {
			return "", err
		}
		b.nukeEOL = time.Now().Add(d)
		b.nukeText = bytes.ToLower(text)
	}
	return "", nil
}

func (b *Bot) handleAegis(m *common.Message, r *bufio.Reader) (string, error) {
	if b.isAdmin(m.Nick) {
		b.nukeEOL = time.Now()
	}
	return "", nil
}

func (b *Bot) handleBans(m *common.Message, r *bufio.Reader) (string, error) {
	return b.toURL("/" + destinyPath + "/" + time.Now().UTC().Format("January 2006") + "/bans.txt"), nil
}

func (b *Bot) handleSubs(m *common.Message, r *bufio.Reader) (string, error) {
	return b.toURL("/" + destinyPath + "/" + time.Now().UTC().Format("January 2006") + "/subscribers.txt"), nil
}

func (b *Bot) handleUptime(m *common.Message, r *bufio.Reader) (string, error) {
	return time.Since(b.start).String(), nil
}
