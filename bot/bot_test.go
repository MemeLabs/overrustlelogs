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
	b = NewBot(common.NewDestinyChat())
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

	rs, err := b.runCommand(b.public, m)
	log.Println(rs)
	if err != nil {
		log.Println("error running logs")
		t.Fail()
	}
	if !strings.Contains(rs, "logs.") {
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

	_, err := b.runCommand(b.private, m)
	if err != nil {
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

	_, err := b.runCommand(b.private, m)
	if err != nil {
		log.Println("error running unignore")
		t.Fail()
	}
	if b.isIgnored("CriminalCutie") {
		log.Println("user still ignored")
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

	_, err := b.runCommand(b.public, m)
	if err != nil {
		log.Println("error running nuke")
		t.Fail()
	}

	m = &common.Message{
		Command: "MSG",
		Nick:    "Destiny",
		Data:    "!log Destiny",
		Time:    time.Now(),
	}

	_, err = b.runCommand(b.public, m)
	if err != ErrNukeTimeout {
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

	_, err := b.runCommand(b.public, m)
	if err != nil {
		log.Println("error running nuke")
		t.Fail()
	}

	m = &common.Message{
		Command: "MSG",
		Nick:    "Destiny",
		Data:    "!log Destiny",
		Time:    time.Now(),
	}

	_, err = b.runCommand(b.public, m)
	if err == ErrNukeTimeout {
		log.Println("failed to unset nuke timeout")
		t.Fail()
	}
}
