package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"

	"github.com/cloudflare/golz4"
)

func main() {
	user := []byte("lirik:")
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
		// size, _ := lz4.CompressHCLevel(data, buf, 16)

		// err = ioutil.WriteFile(baseDir+"/"+strings.Replace(info.Name(), ".txt", ".txt.lz4", -1), buf[:size], 0644)
		// if err != nil {
		// 	log.Println("error writing file %s", err)
		// }

		// c, err := lz4.Encode(nil, data)
		// if err != nil {
		// 	log.Fatalf("error encoding %s", err)
		// }
		// err = ioutil.WriteFile(baseDir+"/"+strings.Replace(info.Name(), ".txt", ".txt.lz4", -1), c, 0644)
		// if err != nil {
		// 	log.Println("error writing file %s", err)
		// }
		// lz4.Decode(buf[:], data)

		// g := bytes.NewReader(buf)

		// 	g, err := gzip.NewReader(f)
		// 	if err != nil {
		// 		log.Fatalf("error creating gzip reader %s", err)
		// 	}

		buf = buf[:]
		lz4.Uncompress(data, buf)

		t := bytes.NewReader(buf)
		r := bufio.NewReaderSize(t, 10*1024*1024)

	ReadLine:
		for {
			line, err := r.ReadSlice('\n')
			if err == io.EOF {
				break
			} else if err != nil {
				log.Fatalf("error reading bytes %s", err)
			}

			for i := 0; i < len(user); i++ {
				if i+26 > len(line) || line[i+26] != user[i] {
					continue ReadLine
				}
			}

			fmt.Print(string(line))
		}
	}
}
