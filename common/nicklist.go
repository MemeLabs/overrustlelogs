package common

import (
	"bytes"
	"io/ioutil"
	"os"

	"github.com/cloudflare/golz4"
)

var empty struct{}

// NickList list of unique nicks
type NickList map[string]struct{}

// Add append nick to list
func (n NickList) Add(nick string) {
	n[nick] = empty
}

// WriteTo writes nicks to the disk
func (n NickList) WriteTo(path string) error {
	buf := bytes.NewBuffer([]byte{})
	for nick := range n {
		buf.WriteString(nick)
		buf.WriteByte(0)
	}
	data := make([]byte, lz4.CompressBound(buf.Bytes()))
	size, err := lz4.CompressHC(buf.Bytes(), data)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path+".writing", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	if _, err := f.Write(data[:size]); err != nil {
		f.Close()
		return err
	}
	f.Close()
	if err := os.Rename(path+".writing", path+".lz4"); err != nil {
		return err
	}
	return nil
}

// ReadFrom adds nicks from the disk
func (n NickList) ReadFrom(path string) error {
	if path[len(path)-4:] != ".lz4" {
		path += ".lz4"
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	data, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}
	buf := make([]byte, 10*1024*1024)
	if err := lz4.Uncompress(data, buf); err != nil {
		return err
	}
	offset := 0
	for i, v := range buf {
		if v == 0 {
			if i == offset {
				break
			}
			n[string(buf[offset:i])] = empty
			offset = i + 1
		}
	}
	return nil
}
