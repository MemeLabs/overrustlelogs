package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"

	"github.com/slugalisk/overrustlelogs/common"
)

type command interface {
	init() error
	exec() error
}

var (
	commands = map[string]command{
		"compress":   &compress{},
		"uncompress": &uncompress{},
		"nicks":      &nicks{},
	}
	c command
)

func init() {
	if len(os.Args) == 0 {
		os.Exit(2)
	}
	var ok bool
	if c, ok = commands[os.Args[1]]; ok {
		if err := c.init(); err != nil {
			fmt.Println(err)
			os.Exit(2)
		}
	} else {
		fmt.Println("invalid command")
		os.Exit(2)
	}
}

func main() {
	if err := c.exec(); err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	os.Exit(1)
}

type compress struct {
	path string
}

func (c *compress) init() error {
	if len(os.Args) < 3 {
		return errors.New("not enough args")
	}
	c.path = os.Args[2]
	return nil
}

func (c *compress) exec() error {
	if _, err := common.CompressFile(c.path); err != nil {
		return err
	}
	return nil
}

type uncompress struct {
	path string
}

func (c *uncompress) init() error {
	if len(os.Args) < 3 {
		return errors.New("not enough args")
	}
	c.path = os.Args[2]
	return nil
}

func (c *uncompress) exec() error {
	if _, err := common.UncompressFile(c.path); err != nil {
		return err
	}
	return nil
}

type nicks struct {
	path string
}

func (c *nicks) init() error {
	if len(os.Args) < 3 {
		return errors.New("not enough args")
	}
	c.path = os.Args[2]
	return nil
}

func (c *nicks) exec() error {
	var data []byte
	data, err := common.ReadCompressedFile(c.path)
	if os.IsNotExist(err) {
		f, err := os.Open(c.path)
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
	if err := nicks.WriteTo(regexp.MustCompile("\\.txt(\\.lz4)?$").ReplaceAllString(c.path, ".nicks")); err != nil {
		return err
	}
	return nil
}

// func batchCompress() {
// 	baseDir := "/var/overrustle/logs/Lirik chatlog/July 2015"

// 	files, err := ioutil.ReadDir(baseDir)
// 	if err != nil {
// 		log.Panicf("error reading logs dir %s", err)
// 	}

// 	buf := make([]byte, 10*1024*1024)
// 	for _, info := range files {
// 		f, err := os.Open(baseDir + "/" + info.Name())
// 		if err != nil {
// 			log.Fatalf("error creating file reader %s", err)
// 		}

// 		data, _ := ioutil.ReadAll(f)
// 		size, _ := lz4.CompressHCLevel(data, buf, 16)

// 		err = ioutil.WriteFile(BaseDir+"/"+strings.Replace(info.Name(), ".txt", ".txt.lz4", -1), buf[:size], 0644)
// 		if err != nil {
//			log.Println("error writing file %s", err)
// 		}
// 	}
// }
