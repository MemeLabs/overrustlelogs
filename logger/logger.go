package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/slugalisk/overrustlelogs/common"
)

func init() {
	configPath := flag.String("config", "", "config path")
	flag.Parse()
	common.SetupConfig(*configPath)
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	logs := NewChatLogs()

	dc := common.NewDestinyChat()
	dl := NewDestinyLogger(logs)
	go dl.Log(dc.Messages())
	go dc.Run()

	tc := common.NewTwitchChat(func(ch string, m chan *common.Message) {
		log.Printf("started logging %s", ch)
		NewTwitchLogger(logs, ch).Log(m)
		log.Printf("stopped logging %s", ch)
	})
	go tc.Run()

	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
	select {
	case <-sigint:
		logs.Close()
		log.Println("i love you guys, be careful")
		os.Exit(0)
	}
}
