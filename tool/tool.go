package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"regexp"

	"github.com/slugalisk/overrustlelogs/common"
)

var commands = map[string]command{
	"compress":   compress,
	"uncompress": uncompress,
	"read":       read,
	"readnicks":  readNicks,
	"nicks":      nicks,
	"readlines":  readLines,
	"indexlines": indexLines,
	"migrate":    migrate,
}

func main() {
	if len(os.Args) < 2 {
		os.Exit(1)
	}
	if c, ok := commands[os.Args[1]]; ok {
		if err := c(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	} else {
		fmt.Println("invalid command")
		os.Exit(1)
	}
	os.Exit(0)
}

type command func() error

func compress() error {
	if len(os.Args) < 3 {
		return errors.New("not enough args")
	}
	path := os.Args[2]
	if _, err := common.CompressFile(path); err != nil {
		return err
	}
	return nil
}

func uncompress() error {
	if len(os.Args) < 3 {
		return errors.New("not enough args")
	}
	path := os.Args[2]
	if _, err := common.UncompressFile(path); err != nil {
		return err
	}
	return nil
}

func nicks() error {
	if len(os.Args) < 3 {
		return errors.New("not enough args")
	}
	path := os.Args[2]
	var data []byte
	data, err := common.ReadCompressedFile(path)
	if os.IsNotExist(err) {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		data, err = ioutil.ReadAll(f)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	r := bufio.NewReaderSize(bytes.NewReader(data), len(data))
	nick := regexp.MustCompile("^\\[[^\\]]+\\]\\s*([a-zA-Z0-9\\_\\-]+):")
	nicks := common.NickList{}
	for {
		line, err := r.ReadSlice('\n')
		if err != nil {
			if err != io.EOF {
				return err
			}
			break
		}
		if ok := nick.Match(line); ok {
			match := nick.FindSubmatch(line)
			nicks.Add(string(match[1]))
		}
	}
	if err := nicks.WriteTo(regexp.MustCompile("\\.txt(\\.lz4)?$").ReplaceAllString(path, ".nicks")); err != nil {
		return err
	}
	return nil
}

func read() error {
	if len(os.Args) < 3 {
		return errors.New("not enough args")
	}
	path := os.Args[2]
	if regexp.MustCompile("\\.txt\\.lz4$").MatchString(path) {
		buf, err := common.ReadCompressedFile(path)
		if err != nil {
			return err
		}
		os.Stdout.Write(buf)
	} else {
		return errors.New("invalid file")
	}
	return nil
}

func readNicks() error {
	if len(os.Args) < 3 {
		return errors.New("not enough args")
	}
	path := os.Args[2]
	if regexp.MustCompile("\\.nicks\\.lz4$").MatchString(path) {
		nicks := common.NickList{}
		if err := common.ReadNickList(nicks, path); err != nil {
			return err
		}
		for nick := range nicks {
			fmt.Println(nick)
		}
	} else {
		return errors.New("invalid file")
	}
	return nil
}

func indexLines() error {
	if len(os.Args) < 3 {
		return errors.New("not enough args")
	}
	path := os.Args[2]
	if regexp.MustCompile("\\.txt\\.lz4$").MatchString(path) {
		buf, err := common.ReadCompressedFile(path)
		if err != nil {
			return err
		}
		reader := bufio.NewReaderSize(bytes.NewReader(buf), len(buf))
		index := &common.LineIndex{}
		for {
			line, err := reader.ReadSlice('\n')
			if err != nil {
				if err != io.EOF {
					log.Printf("error reading bytes %s", err)
				}
				break
			}
			index.Add(uint16(len(line)))
		}
		if err := index.WriteTo(regexp.MustCompile("\\.txt(\\.lz4)?$").ReplaceAllString(path, ".index")); err != nil {
			return err
		}
	} else {
		return errors.New("invalid file")
	}
	return nil
}

func readLines() error {
	if len(os.Args) < 3 {
		return errors.New("not enough args")
	}
	path := os.Args[2]
	if regexp.MustCompile("\\.index\\.lz4$").MatchString(path) {
		index := &common.LineIndex{}
		if err := common.ReadLineIndex(index, path); err != nil {
			return err
		}
		for _, v := range *index {
			fmt.Println(v)
		}
	} else {
		return errors.New("invalid file")
	}
	return nil
}
