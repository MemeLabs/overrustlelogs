package common

import (
	"bufio"
	"bytes"
	"io"
	"log"
	"testing"
	"time"
)

const LogLinePrefixLength = 26

func TestLineIndexRead(t *testing.T) {
	data, _ := ReadCompressedFile("../test/sample.txt")
	filter := nickFilter("Destiny")
	for i := 0; i < 10; i++ {
		count := 0
		start := time.Now()
		r := bufio.NewReaderSize(bytes.NewReader(data), len(data))
		for {
			line, err := r.ReadSlice('\n')
			if err != nil {
				if err != io.EOF {
					log.Printf("error reading bytes %s", err)
				}
				break
			}
			if filter(line) {
				count++
			}
		}
		log.Println(count, time.Since(start))
	}

	index := LineIndex{}
	ReadLineIndex(&index, "../test/sample.index")
	log.Println("----------------------")
	for i := 0; i < 10; i++ {
		count := 0
		start := time.Now()
		offset := 0
		for j := 0; j < len(index); j++ {
			line := data[offset : offset+int(index[j])]
			offset += int(index[j])

			if filter(line) {
				count++
			}
		}
		log.Println(count, time.Since(start))
	}
}

func TestLineIndexIO(t *testing.T) {
	for i := 0; i < 10; i++ {
		start := time.Now()
		ReadCompressedFile("../test/sample.txt")
		log.Println(time.Since(start))
	}
	log.Println("----------------------")
	for i := 0; i < 10; i++ {
		start := time.Now()
		index := LineIndex{}
		ReadLineIndex(&index, "../test/sample.index")
		log.Println(time.Since(start))
	}
}

func nickFilter(nick string) func([]byte) bool {
	nick += ":"
	return func(line []byte) bool {
		for i := 0; i < len(nick); i++ {
			if i+LogLinePrefixLength > len(line) || line[i+LogLinePrefixLength] != nick[i] {
				return false
			}
		}
		return true
	}
}
