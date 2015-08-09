package main

import (
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/cloudflare/golz4"
)

// temp ish.. move to config
const (
	// BaseDir    = "/var/overrustle/logs"
	BaseDir = "/var/www/public/_public"
)

func batchCompress() {
	baseDir := "/var/overrustle/logs/Lirik chatlog/July 2015"

	files, err := ioutil.ReadDir(baseDir)
	if err != nil {
		log.Panicf("error reading logs dir %s", err)
	}

	buf := make([]byte, 10*1024*1024)
	for _, info := range files {
		f, err := os.Open(baseDir + "/" + info.Name())
		if err != nil {
			log.Fatalf("error creating file reader %s", err)
		}

		data, _ := ioutil.ReadAll(f)
		size, _ := lz4.CompressHCLevel(data, buf, 16)

		err = ioutil.WriteFile(BaseDir+"/"+strings.Replace(info.Name(), ".txt", ".txt.lz4", -1), buf[:size], 0644)
		if err != nil {
			log.Println("error writing file %s", err)
		}
	}
}
