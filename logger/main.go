package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/slugalisk/overrustlelogs/chat"
	"github.com/slugalisk/overrustlelogs/common"
)

func init() {
	configPath := flag.String("config", "", "config path")
	flag.Parse()
	common.SetupConfig(*configPath)
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	logs := NewChatLogs()

	dc := chat.NewDestiny()
	dl := NewLogger(logs)
	go dl.DestinyLog(dc.Messages())
	go dc.Run()

	twitchLogHandler := func(m <-chan *common.Message) {
		tLogs := NewChatLogs()
		NewLogger(tLogs).TwitchLog(m)
	}

	orl := NewTwitchLogger(twitchLogHandler)
	orl.Start()

	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
	select {
	case <-sigint:
		logs.Close()
		dc.Stop()
		orl.Stop()
		log.Println("i love you guys, be careful")
		os.Exit(0)
	}
}
