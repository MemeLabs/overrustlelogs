package common

import (
	"bytes"
	"io"
	"os"
	"strings"
	"time"
)

var empty struct{}

// NickStore nick list data
type NickStore interface {
	Add(string)
	Remove(string)
}

// ReadNickList adds nicks from the disk
func ReadNickList(n NickStore, path string) error {
	buf, err := ReadCompressedFile(path)
	if err != nil {
		return err
	}
	offset := 0
	for i, v := range buf {
		if v == 0 {
			n.Add(string(buf[offset:i]))
			offset = i + 1
		}
	}
	return nil
}

// NickList list of unique nicks
type NickList map[string]struct{}

// Add append nick to list
func (n NickList) Add(nick string) {
	n[nick] = empty
}

// Remove deletes a nick from the list
func (n NickList) Remove(nick string) {
	delete(n, nick)
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

// NickListLower lower case nick list for case insensitive search
type NickListLower map[string]struct{}

// Add implement NickStore
func (n NickListLower) Add(nick string) {
	n[strings.ToLower(nick)] = empty
}

// Remove deletes a nick from the list
func (n NickListLower) Remove(nick string) {
	delete(n, strings.ToLower(nick))
}

// NickCaseMap map from lower case nicks
type NickCaseMap map[string]string

// Add implement NickStore
func (n NickCaseMap) Add(nick string) {
	n[strings.ToLower(nick)] = nick
}

// Remove deletes a nick from the list
func (n NickCaseMap) Remove(nick string) {
	delete(n, strings.ToLower(nick))
}

// NickSearch scans nick indexes in reverse chronological order
type NickSearch struct {
	nick   string
	path   string
	months map[string]struct{}
	date   time.Time
}

// NewNickSearch create scanner
func NewNickSearch(path string, nick string) (*NickSearch, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	names, err := f.Readdirnames(0)
	if err != nil {
		return nil, err
	}
	months := make(map[string]struct{}, len(names))
	for _, name := range names {
		months[name] = struct{}{}
	}
	return &NickSearch{
		nick:   strings.ToLower(nick),
		path:   path,
		months: months,
		date:   time.Now().UTC().Add(24 * time.Hour),
	}, nil
}

// Next find next occurrence
func (n *NickSearch) Next() (*NickSearchResult, error) {
	for {
		n.date = n.date.Add(-24 * time.Hour)
		if _, ok := n.months[n.date.Format("January 2006")]; !ok {
			return nil, io.EOF
		}
		nicks := NickCaseMap{}
		ReadNickList(nicks, n.path+n.date.Format("/January 2006/2006-01-02")+".nicks")
		if nick, ok := nicks[n.nick]; ok {
			return &NickSearchResult{nick, n.date}, nil
		}
	}
}

// NickSearchResult nick/path data
type NickSearchResult struct {
	nick string
	date time.Time
}

// Nick case corrected nick
func (n *NickSearchResult) Nick() string {
	return n.nick
}

// Month path string
func (n *NickSearchResult) Month() string {
	return n.date.Format("January 2006")
}

// Day path string
func (n *NickSearchResult) Day() string {
	return n.date.Format("2006-01-02")
}

// Date time object
func (n *NickSearchResult) Date() time.Time {
	return n.date
}
