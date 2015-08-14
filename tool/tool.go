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

var commands = map[string]command{
	"compress":   compress,
	"uncompress": uncompress,
	"nicks":      nicks,
}

func main() {
	if len(os.Args) < 2 {
		os.Exit(2)
	}
	if c, ok := commands[os.Args[1]]; ok {
		if err := c(); err != nil {
			fmt.Println(err)
			os.Exit(2)
		}
	} else {
		fmt.Println("invalid command")
		os.Exit(2)
	}
	os.Exit(1)
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
