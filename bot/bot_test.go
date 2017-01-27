package main

import (
	"log"
	"strings"
	"testing"
	"time"

	"github.com/slugalisk/overrustlelogs/common"
)

var b *Bot

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	// common.SetupConfig("E:\\orl\\overrustlelogswin.conf")
	b = NewBot(common.NewDestiny())
}

func TestIsAdmin(t *testing.T) {
	if !b.isAdmin("Destiny") {
		log.Println("user not ignored")
		t.Fail()
	}
}

func TestLogs(t *testing.T) {
	m := &common.Message{
		Command: "MSG",
		Nick:    "Destiny",
		Data:    "!log destiny",
		Time:    time.Now(),
	}

	if rs, err := b.runCommand(b.public, m); err != nil {
		log.Println("error running logs", err)
		t.Fail()
	} else if !strings.Contains(rs, "logs") {
		log.Printf("invalid log response \"%s\"", rs)
		t.Fail()
	}
}

func TestIgnore(t *testing.T) {
	m := &common.Message{
		Command: "PRIVMSG",
		Nick:    "Destiny",
		Data:    "!ignore CriminalCutie",
		Time:    time.Now(),
	}

	if _, err := b.runCommand(b.private, m); err != nil {
		log.Println("error running ignore")
		t.Fail()
	}
	if !b.isIgnored("CriminalCutie") {
		log.Println("user not ignored")
		t.Fail()
	}
}

func TestUnignore(t *testing.T) {
	m := &common.Message{
		Command: "PRIVMSG",
		Nick:    "Destiny",
		Data:    "!unignore CriminalCutie",
		Time:    time.Now(),
	}

	if _, err := b.runCommand(b.private, m); err != nil {
		log.Println("error running unignore")
		t.Fail()
	}
	if b.isIgnored("CriminalCutie") {
		log.Println("user still ignored")
		t.Fail()
	}
}

func TestMuteAdd(t *testing.T) {
	m := &common.Message{
		Command: "PRIVMSG",
		Nick:    "Destiny",
		Data:    "!add cam",
		Time:    time.Now(),
	}

	if _, err := b.runCommand(b.public, m); err != nil {
		log.Println("error running add")
		t.Fail()
	}
	if !b.isInAutoMute("cam") {
		log.Println("cam not not muted")
		t.Fail()
	}
}

func TestMuteDel(t *testing.T) {
	m := &common.Message{
		Command: "PRIVMSG",
		Nick:    "Destiny",
		Data:    "!del cam",
		Time:    time.Now(),
	}

	if _, err := b.runCommand(b.public, m); err != nil {
		log.Println("error running del")
		t.Fail()
	}
	if b.isInAutoMute("cam") {
		log.Println("cam is still in automute")
		t.Fail()
	}
}

func TestMute(t *testing.T) {
	m := &common.Message{
		Command: "PRIVMSG",
		Nick:    "Destiny",
		Data:    "!add sta",
		Time:    time.Now(),
	}

	if _, err := b.runCommand(b.public, m); err != nil {
		log.Println("error running mute add")
		t.Fail()
	}

	if !b.isInAutoMute("dgg.overrustlelogs.net/stank") {
		t.Fail()
	}
}

func TestNuke(t *testing.T) {
	m := &common.Message{
		Command: "MSG",
		Nick:    "Destiny",
		Data:    "!nuke overrustle",
		Time:    time.Now(),
	}

	if _, err := b.runCommand(b.public, m); err != nil {
		log.Println("error running nuke")
		t.Fail()
	}

	m = &common.Message{
		Command: "MSG",
		Nick:    "Destiny",
		Data:    "!log Destiny",
		Time:    time.Now(),
	}

	rs, _ := b.runCommand(b.public, m)
	if !b.isNuked(rs) {
		log.Println("failed to set nuke timeout")
		t.Fail()
	}
}

func TestAegis(t *testing.T) {
	m := &common.Message{
		Command: "MSG",
		Nick:    "Destiny",
		Data:    "!aegis",
		Time:    time.Now(),
	}

	if _, err := b.runCommand(b.public, m); err != nil {
		log.Println("error running nuke")
		t.Fail()
	}

	m = &common.Message{
		Command: "MSG",
		Nick:    "Destiny",
		Data:    "!log Destiny",
		Time:    time.Now(),
	}

	rs, _ := b.runCommand(b.public, m)
	if b.isNuked(rs) {
		log.Println("failed to unset nuke timeout")
		t.Fail()
	}
}
