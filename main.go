package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"runtime"

	"github.com/slugalisk/overrustlelogs/common"
	"github.com/slugalisk/overrustlelogs/logger"
	"github.com/slugalisk/overrustlelogs/server"
)

var (
	startServer *bool
	startLogger *bool
)

func init() {
	configPath := flag.String("config", "", "config path")
	startServer = flag.Bool("server", false, "start server")
	startLogger = flag.Bool("logger", false, "start logger")
	flag.Parse()
	common.SetupConfig(*configPath)
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	if *startServer {
		server.Start()
	}

	if *startLogger {
		logger.Start()
	}

	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt)
	select {
	case <-sigint:
		log.Println("i love you guys, be careful")
	}
}
