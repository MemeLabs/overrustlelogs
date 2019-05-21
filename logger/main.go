package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/MemeLabs/overrustlelogs/common"
)

func init() {
	configPath := flag.String("config", "", "config path")
	flag.Parse()
	common.SetupConfig(*configPath)
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	logs := NewChatLogs()

	// cfg := common.GetConfig().DestinyGG
	// dc := common.NewDestiny("Destinygg", cfg.OriginURL, cfg.Cookie, cfg.SocketURL, false)
	vc := common.NewDestiny("Vaushgg", "https://www.vaush.gg", "", "wss://www.vaush.gg/ws", true)
	dl := NewLogger(logs)
	// go dl.DestinyLog(dc.Messages())
	go dl.DestinyLog(vc.Messages())
	// go dc.Run()
	go vc.Run()

	// twitchLogHandler := func(m <-chan *common.Message) {
	// 	NewLogger(NewChatLogs()).TwitchLog(m)
	// }

	// tl := NewTwitchLogger(twitchLogHandler)
	// go tl.Start()

	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
	<-sigint
	logs.Close()
	// dc.Stop()
	// tl.Stop()
	vc.Stop()
	log.Println("i love you guys, be careful")
	os.Exit(0)
}
