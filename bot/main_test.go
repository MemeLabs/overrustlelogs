package main

import (
	"log"
	"strings"
	"testing"
	"time"

	"github.com/MemeLabs/overrustlelogs/common"
)

var b *Bot

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	b = NewBot(common.NewDestiny())
}

func TestIsAdmin(t *testing.T) {
	if !b.isAdmin("Destiny") {
		t.Error("user not ignored")
	}
}

func TestLogs(t *testing.T) {
	m := &common.Message{
		Type: "MSG",
		Nick: "Destiny",
		Data: "!log destiny",
		Time: time.Now(),
	}

	rs, err := b.runCommand(b.public, m)
	if err != nil {
		t.Error("error running logs", err)
	} else if !strings.Contains(rs, "logs") {
		t.Errorf("invalid log response \"%s\"", rs)
	}
}

func TestTlogs(t *testing.T) {
	m := &common.Message{
		Type: "MSG",
		Nick: "Destiny",
		Data: "!tlog Destiny",
		Time: time.Now(),
	}
	want := "ttv.overrustlelogs.net/Destiny/destiny"
	if rs, err := b.runCommand(b.public, m); err != nil {
		t.Errorf("error running tlogs %s", err.Error())
	} else if !strings.Contains(rs, want) {
		t.Errorf("invalid log response, got: %s; want: %s;", rs, want)
	}
}

func TestMentions(t *testing.T) {
	tests := []*common.Message{
		{"MSG", "", "Destiny", "!mentions", time.Now()},
		{"MSG", "", "Destiny", "!mentions 2017-01-10", time.Now()},
		{"MSG", "", "Destiny", "!mentions 01-02-2017", time.Now()},
		{"MSG", "", "Destiny", "!mentions 3000-01-10", time.Now()},
	}
	expected := []string{
		"Destiny dgg.overrustlelogs.net/mentions/Destiny",
		"Destiny dgg.overrustlelogs.net/mentions/Destiny?date=2017-01-10",
		"Destiny dgg.overrustlelogs.net/mentions/Destiny",
		"Destiny BASEDWATM8 i can't look into the future.",
	}

	for i, test := range tests {
		if got, err := b.runCommand(b.public, test); err != nil {
			t.Error("error running tlogs", err)
		} else if got != expected[i] {
			t.Errorf("invalid log response, got: %s; want: %s", got, expected[i])
		}
	}
}

func TestMentionsFail(t *testing.T) {
	tests := []*common.Message{
		{"MSG", "", "Destiny", time.Now().Add(24 * time.Hour).Format("!mentions 2006-01-02"), time.Now()},
	}
	expected := []string{
		"Destiny BASEDWATM8 i can't look into the future.",
	}
	for i, test := range tests {
		got, err := b.runCommand(b.public, test)
		if err != nil {
			t.Errorf("error running tlogs, %s", err.Error())
		}
		if got != expected[i] {
			t.Errorf("invalid log response, got: %s; want: %s;", got, expected[i])
		}
		// log.Printf("invalid log response \"%s\"", rs)
	}
}

func TestIgnore(t *testing.T) {
	m := &common.Message{
		Type: "PRIVMSG",
		Nick: "Destiny",
		Data: "!ignore CriminalCutie",
		Time: time.Now(),
	}

	if _, err := b.runCommand(b.private, m); err != nil {
		t.Error("error running ignore")
	}
	if !b.isIgnored("CriminalCutie") {
		t.Error("user not ignored")
	}
}

func TestUnignore(t *testing.T) {
	m := &common.Message{
		Type: "PRIVMSG",
		Nick: "Destiny",
		Data: "!unignore CriminalCutie",
		Time: time.Now(),
	}

	if _, err := b.runCommand(b.private, m); err != nil {
		t.Error("error running unignore")
	}
	if b.isIgnored("CriminalCutie") {
		t.Error("user still ignored")
	}
}

func TestIgnoreLog(t *testing.T) {
	m := &common.Message{
		Type: "PRIVMSG",
		Nick: "dbc",
		Data: "!ignorelog CriminalCutie",
		Time: time.Now(),
	}

	if _, err := b.runCommand(b.private, m); err != nil {
		t.Error("error running ignore")
	}
	if !b.isLogIgnored("CriminalCutie") {
		t.Error("userlog not ignored")
	}
}

func TestUnignoreLog(t *testing.T) {
	m := &common.Message{
		Type: "PRIVMSG",
		Nick: "dbc",
		Data: "!unignorelog CriminalCutie",
		Time: time.Now(),
	}

	if _, err := b.runCommand(b.private, m); err != nil {
		t.Error("error running unignore")
	}
	if b.isIgnored("CriminalCutie") {
		t.Error("userlog still ignored")
	}
}

func TestMuteAdd(t *testing.T) {
	m := &common.Message{
		Type: "PRIVMSG",
		Nick: "Destiny",
		Data: "!add cam",
		Time: time.Now(),
	}

	if _, err := b.runCommand(b.public, m); err != nil {
		t.Error("error running add")
	}
	if !b.isInAutoMute("cam") {
		t.Error("cam not not muted")
	}
}

func TestMuteDel(t *testing.T) {
	m := &common.Message{
		Type: "PRIVMSG",
		Nick: "Destiny",
		Data: "!del cam",
		Time: time.Now(),
	}

	if _, err := b.runCommand(b.public, m); err != nil {
		t.Error("error running del")
	}
	if b.isInAutoMute("cam") {
		t.Error("cam is still in automute")
	}
}

func TestMute(t *testing.T) {
	m := &common.Message{
		Type: "PRIVMSG",
		Nick: "Destiny",
		Data: "!add sta",
		Time: time.Now(),
	}

	if _, err := b.runCommand(b.public, m); err != nil {
		t.Error("error running mute add")
	}

	if !b.isInAutoMute("dgg.overrustlelogs.net/stank") {
		t.Error("failed to set automute")
	}
}

func TestNuke(t *testing.T) {
	m := &common.Message{
		Type: "MSG",
		Nick: "Destiny",
		Data: "!nuke stiny",
		Time: time.Now(),
	}

	if _, err := b.runCommand(b.public, m); err != nil {
		t.Error("error running nuke")
	}

	if !b.isNuked("January 2017. dgg.overrustlelogs.net/Destiny") {
		t.Error("failed to set nuke timeout")
	}
}

func TestAegis(t *testing.T) {
	m := &common.Message{
		Type: "MSG",
		Nick: "Destiny",
		Data: "!aegis",
		Time: time.Now(),
	}

	if _, err := b.runCommand(b.public, m); err != nil {
		t.Error("error running nuke")
	}

	m = &common.Message{
		Type: "MSG",
		Nick: "Destiny",
		Data: "!log Destiny",
		Time: time.Now(),
	}

	rs, _ := b.runCommand(b.public, m)
	if b.isNuked(rs) {
		t.Error("failed to unset nuke timeout")
	}
}
