package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	s "strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type WSConn struct {
	sync.Mutex
	conn   *websocket.Conn
	dialer websocket.Dialer
	heads  http.Header
}

type Message struct {
	Nick      string   `json:"nick"`
	NickLower string   `json:"nicklower"`
	Features  []string `json:"features"`
	Data      string   `json:"data"`
	Timestamp int64    `json:"timestamp"`
	Command   string   `json:"command"`
}

var (
	url = "ws://destiny.gg:9998/ws"
	origin = []string{"http://destiny.gg"}
)

func (w *WSConn) Reconnect() {
	if w.conn != nil {
		w.Lock()
		_ = w.conn.Close()
		w.Unlock()
	}

	var err error
	w.conn, _, err = w.dialer.Dial(url, w.heads)
	if err != nil {
		logAndDelay(err)
		w.Reconnect()
		return
	}
}

func (w *WSConn) Read() Message {
	err := w.conn.SetReadDeadline(time.Now().Add(20 * time.Second))
	if err != nil {
		w.Reconnect()
		w.Read()
	}

	_, msg, err := w.conn.ReadMessage()
	if err != nil {
		logAndDelay(err)
		w.Reconnect()

	}
	return parse(msg)
}

func (w *WSConn) Run(u string, ch chan string) {
	var chLog = make(chan Message, 10)

	go WriteFile(chLog)

	log.Println("Connecting to", u)
	w.Reconnect()
	log.Println("Connected to", u)

	for {
		msg := w.Read()
		chLog <- msg
	}
	ch <- u
}

func (w *WSConn) Write(msgType, msg string) {
	err := w.conn.WriteMessage(1, []byte(msgType+` {"data":"`+msg+`"}`))
	if err != nil {
		logAndDelay(err)
		w.Reconnect()
	}
}

func (w *WSConn) WritePrivate(msgType, u, msg string) {
	w.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	err := w.conn.WriteMessage(1, []byte(msgType+` {"nick":"`+u+`", "data":"`+msg+`"}`))
	if err != nil {
		logAndDelay(err)
		w.Reconnect()
	}
}

func parse(msg []byte) (m Message) {
	list := s.SplitAfterN(string(msg), " ", 2)
	if len(list) != 2 {
		return Message{}
	}

	err := json.Unmarshal([]byte(list[1]), &m)
	if err != nil {
		return Message{}
	}

	m.Command = s.Trim(list[0], " ")
	m.Data = s.Replace(m.Data, "\n", "", -1)
	m.NickLower = s.ToLower(m.Nick)
	return m
}

func logAndDelay(err error) {
	log.Printf("Connection failed ERROR: %s\n", err)
	log.Println("Reconnecting in 1 Seconds...")
	time.Sleep(1 * time.Second)
}

func slurpFile(fn string) []string {
	d, err := ioutil.ReadFile(fn)
	if err != nil {
		LogErr(err)
		return []string{}
	}
	dl := s.Split(string(d), ",")
	var dn []string
	for _, v := range dl {
		if v != "" {
			dn = append(dn, v)
		}
	}
	return dn
}

func writeFile(fn string, s []string) {
	var d string

	for _, v := range s {
		if v != "" {
			d += v + ","
		}
	}

	f, err := os.Create(fn)
	if err != nil {
		LogErr(err)
		return
	}
	defer f.Close()
	f.WriteString(d)
}

func remove(n string, sl []string) []string {
	mu := sync.Mutex{}
	for i, data := range sl {
		if s.EqualFold(n, data) {
			mu.Lock()
			sl = append(sl[:i], sl[i+1:]...)
			mu.Unlock()
			return sl
		}
	}
	return sl
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	quit := make(chan string, 1)

	wsCom := &WSConn{
		dialer: websocket.Dialer{HandshakeTimeout: 10 * time.Second},
		heads:  http.Header{"Origin": origin},
	}

	go wsCom.Run("Destinygg", quit)

	for {
		ch := <-quit
		if ch != "" {
			log.Println("Restarting", ch, "...")
			go wsCom.Run(ch, quit)
		}
	}
}
