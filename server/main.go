package main

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
)

func main() {
	user := []byte("lirik")
	baseDir := "/var/overrustle/Lirik chatlog/July 2015"

	files, err := ioutil.ReadDir(baseDir)
	if err != nil {
		log.Panicf("error reading logs dir %s", err)
	}

	for _, info := range files {
		f, err := os.Open(baseDir + "/" + info.Name())
		if err != nil {
			log.Fatalf("error creating file reader %s", err)
		}

		g, err := gzip.NewReader(f)
		if err != nil {
			log.Fatalf("error creating gzip reader %s", err)
		}

		r := bufio.NewReaderSize(g, 512)

	ReadLine:
		for {
			line, err := r.ReadSlice('\n')
			if err == io.EOF {
				break
			} else if err != nil {
				log.Fatalf("error reading byts %s", err)
			}

			for i := 0; i < len(user); i++ {
				if line[i+27] != user[i] {
					continue ReadLine
				}
			}

			fmt.Print(string(line))
		}
	}
}
