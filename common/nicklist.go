package common

import (
	"bytes"
	"os"
	"strings"
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
	f, err := WriteCompressedFile(path+".writing", buf.Bytes())
	if err != nil {
		return err
	}
	if err := os.Rename(f.Name(), strings.Replace(f.Name(), ".writing", "", -1)); err != nil {
		return err
	}
	return nil
}

// ReadFrom adds nicks from the disk
func (n NickList) ReadFrom(path string, transform ...func(string) string) error {
	buf, err := ReadCompressedFile(path)
	if err != nil {
		return err
	}
	offset := 0
	for i, v := range buf {
		if v == 0 {
			if i == offset {
				break
			}
			nick := string(buf[offset:i])
			for i := 0; i < len(transform); i++ {
				nick = transform[i](nick)
			}
			n[nick] = empty
			offset = i + 1
		}
	}
	return nil
}
