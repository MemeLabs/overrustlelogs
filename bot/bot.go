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
	ignoreLog   map[string]struct{}
}

// NewBot ...
func NewBot(c *common.DestinyChat) *Bot {
	b := &Bot{
		c:         c,
		stop:      make(chan bool),
		start:     time.Now(),
		admins:    make(map[string]struct{}, len(common.GetConfig().Bot.Admins)),
		ignoreLog: make(map[string]struct{}),
	}
	for _, admin := range common.GetConfig().Bot.Admins {
		b.admins[admin] = struct{}{}
	}
	b.public = map[string]command{
		"log":   b.handleDestinyLogs,
		"tlog":  b.handleTwitchLogs,
		"nuke":  b.handleSimpleNuke,
		"aegis": b.handleAegis,
		"bans":  b.handleBans,
		"subs":  b.handleSubs,
	}
	b.private = map[string]command{
		"log":         b.handleDestinyLogs,
		"tlog":        b.handleTwitchLogs,
		"p":           b.handlePremiumLog,
		"uptime":      b.handleUptime,
		"ignore":      b.handleIgnore,
		"unignore":    b.handleUnignore,
		"ignorelog":   b.handleIgnoreLog,
		"unignorelog": b.handleUnignoreLog,
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
	if d, err := ioutil.ReadFile(common.GetConfig().Bot.IgnoreLogListPath); err == nil {
		ignoreLog := []string{}
		if err := json.Unmarshal(d, &ignoreLog); err == nil {
			for _, nick := range ignoreLog {
				b.addIgnoreLog(nick)
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
			switch m.Command {
			case "MSG":
				if rs, err := b.runCommand(b.public, m); err == nil && rs != "" {
					isAdmin := b.isAdmin(m.Nick)
					if b.isNuked(rs) {
						b.addIgnore(m.Nick)
					} else if isAdmin || (rs != b.lastLine && time.Now().After(b.cooldownEOL)) {
						// NOTE if Destiny requests a log it's pretty SWEATSTINY, so let's add SWEATSTINY at the end of the message :^)
						if m.Nick == "Destiny" {
							rs += " SWEATSTINY"
						}
						if isAdmin && b.lastLine == rs {
							rs += " ."
							if err = b.c.Write(rs); err != nil {
								log.Println(err)
							}
						} else if err = b.c.Write(rs); err != nil {
							log.Println(err)
						}
						b.cooldownEOL = time.Now().Add(cooldownDuration)
						b.lastLine = rs
					}
				} else if err != nil {
					log.Println(err)
				}
			case "PRIVMSG":
				if rs, err := b.runCommand(b.private, m); err == nil && rs != "" {
					if err = b.c.WritePrivate(m.Nick, rs); err != nil {
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
	ignoreLog := []string{}
	for nick := range b.ignoreLog {
		ignoreLog = append(ignoreLog, nick)
	}
	data, _ = json.Marshal(ignoreLog)
	if err := ioutil.WriteFile(common.GetConfig().Bot.IgnoreLogListPath, data, 0644); err != nil {
		log.Fatalf("unable to write ignorelog list %s", err)
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
		c = strings.ToLower(c)
		for cs, cmd := range commands {
			if strings.Index(c, cs) == 0 {
				return cmd(m, r)
			}
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

func (b *Bot) isLogIgnored(nick string) bool {
	_, ok := b.ignoreLog[strings.ToLower(nick)]
	return ok
}

func (b *Bot) addIgnore(nick string) {
	b.ignore[strings.ToLower(nick)] = struct{}{}
}

func (b *Bot) removeIgnore(nick string) {
	delete(b.ignore, strings.ToLower(string(nick)))
}

func (b *Bot) addIgnoreLog(nick string) {
	b.ignoreLog[strings.ToLower(nick)] = struct{}{}
}

func (b *Bot) removeIgnoreLog(nick string) {
	delete(b.ignoreLog, strings.ToLower(string(nick)))
}

func (b *Bot) toURL(host string, path string) string {
	var u, err = url.Parse(host)
	if err != nil {
		log.Fatalf("error parsing configured log host %s", err)
	}
	u.Scheme = ""
	u.Path = path
	return u.String()[2:]
}

func (b *Bot) handlePremiumLog(m *common.Message, r *bufio.Reader) (string, error) {
	return b.toURL(common.GetConfig().LogHost, "/"+destinyPath+"/premium/"+m.Nick+"/"+time.Now().UTC().Format("January 2006")+".txt"), nil
}

func (b *Bot) handleIgnoreLog(m *common.Message, r *bufio.Reader) (string, error) {
	if b.isAdmin(m.Nick) {
		nick, err := ioutil.ReadAll(r)
		if err != nil || !validNick.Match(nick) {
			return "Invalid nick", err
		}
		b.addIgnoreLog(string(nick))
	}
	return "", nil
}

func (b *Bot) handleUnignoreLog(m *common.Message, r *bufio.Reader) (string, error) {
	if b.isAdmin(m.Nick) {
		nick, err := ioutil.ReadAll(r)
		if err != nil || !validNick.Match(nick) {
			return "Invalid nick", err
		}
		if b.isLogIgnored(string(nick)) {
			b.removeIgnoreLog(string(nick))
		}
	}
	return "", nil
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
		if b.isIgnored(string(nick)) {
			b.removeIgnore(string(nick))
		}
	}
	return "", nil
}

func (b *Bot) handleDestinyLogs(m *common.Message, r *bufio.Reader) (string, error) {
	rs, s, err := b.searchNickFromLine(destinyPath, r)
	if err != nil {
		return s, err
	}

	if rs != nil {
		return rs.Month() + " logs. " + b.toURL(common.GetConfig().DestinyGG.LogHost, "/"+rs.Nick()), nil
	}
	return b.toURL(common.GetConfig().DestinyGG.LogHost, "/"), nil
}

func (b *Bot) handleTwitchLogs(m *common.Message, r *bufio.Reader) (string, error) {
	rs, s, err := b.searchNickFromLine(twitchPath, r)
	if err != nil {
		return s, err
	}

	if rs != nil {
		return rs.Month() + " logs. " + b.toURL(common.GetConfig().Twitch.LogHost, "/Destiny/"+rs.Nick()), nil
	}
	return b.toURL(common.GetConfig().Twitch.LogHost, "/Destiny"), nil
}

func (b *Bot) searchNickFromLine(path string, r *bufio.Reader) (*common.NickSearchResult, string, error) {
	nick, err := r.ReadString(' ')
	nick = strings.TrimSpace(nick)
	if (err != nil && err != io.EOF) || len(nick) < 1 || b.isLogIgnored(nick) {
		return nil, "", nil
	}
	if !validNick.Match([]byte(nick)) {
		return nil, "", ErrInvalidNick
	}
	s, err := common.NewNickSearch(common.GetConfig().LogPath+"/"+path, string(nick))
	if err != nil {
		return nil, "", err
	}
	rs, err := s.Next()
	if err != nil {
		return nil, "No logs found for that user.", err
	}

	return rs, "", nil
}

func (b *Bot) handleSimpleNuke(m *common.Message, r *bufio.Reader) (string, error) {
	return b.handleNuke(m, defaultNukeDuration, r)
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
	return b.toURL(common.GetConfig().DestinyGG.LogHost, "/Ban"), nil
}

func (b *Bot) handleSubs(m *common.Message, r *bufio.Reader) (string, error) {
	return b.toURL(common.GetConfig().DestinyGG.LogHost, "/Subscriber"), nil
}

func (b *Bot) handleUptime(m *common.Message, r *bufio.Reader) (string, error) {
	return time.Since(b.start).String(), nil
}
