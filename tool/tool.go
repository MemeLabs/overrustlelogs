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
	"strings"
	"time"

	"github.com/slugalisk/overrustlelogs/common"
)

var commands = map[string]command{
	"compress":   compress,
	"uncompress": uncompress,
	"read":       read,
	"readnicks":  readNicks,
	"nicks":      nicks,
	"migrate":    migrate,
	"namechange": namechange,
}

func main() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)
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

func namechange() error {
	if len(os.Args) < 5 {
		return errors.New("not enough args")
	}
	validNick := regexp.MustCompile("^[a-zA-Z0-9_]+$")
	log := os.Args[2]
	oldName := os.Args[3]
	if !validNick.Match([]byte(oldName)) {
		return errors.New("the old name is not a valid nick")
	}
	newName := os.Args[4]

	replacer := strings.NewReplacer(
		"] "+oldName+":", "] "+newName+":",
		" "+oldName+" ", " "+newName+" ",
		" "+oldName+"\n", " "+newName+"\n",
	)

	log = strings.Replace(log, "txt", "nicks", 1)

	if strings.Contains(log, time.Now().UTC().Format("2006-01-02")) {
		return errors.New("can't modify todays log file")
	}
	fmt.Println(log)

	n := common.NickList{}
	err := common.ReadNickList(n, log)
	if err != nil {
		fmt.Println(err)
		return err
	}

	if _, ok := n[newName]; ok {
		return errors.New("nick already used, choose another one")
	}
	if _, ok := n[oldName]; !ok {
		return errors.New("nick not found")
	}
	n.Remove(oldName)
	n.Add(newName)
	err = n.WriteTo(log[:len(log)-4])
	if err != nil {
		fmt.Println(err)
		return err
	}

	log = strings.Replace(log, "nicks", "txt", 1)

	d, err := common.ReadCompressedFile(log)
	if err != nil {
		fmt.Println(err)
		return err
	}

	newData := []byte(replacer.Replace(string(d)))
	f, err := common.WriteCompressedFile(log, newData)
	if err != nil {
		fmt.Println(err)
		return err
	}
	fmt.Println("replaced nicks in", f.Name())
	f.Close()
	return nil
}
