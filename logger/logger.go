package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

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

	dc := common.NewDestiny()
	dl := NewLogger(logs)
	go dl.DestinyLog(dc.Messages())
	go dc.Run()

	twitchLogHandler := func(m <-chan *common.Message) {
		NewLogger(NewChatLogs()).TwitchLog(m)
	}

	orl := NewTwitchLogger(twitchLogHandler)
	go orl.Start()

	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
	<-sigint
	logs.Close()
	dc.Stop()
	orl.Stop()
	log.Println("i love you guys, be careful")
	os.Exit(0)
}
