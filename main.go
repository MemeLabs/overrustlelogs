package main

import (
	"encoding/json"
	"log"
	"net/http"
	s "strings"
	"time"

	"github.com/gorilla/websocket"
)

type WSConn struct {
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
	url       = "ws://destiny.gg:9998/ws"
	authToken = []string{"authtoken=1234567890"}
	origin    = []string{"http://destiny.gg"}
)

func (w *WSConn) Reconnect() {
	if w.conn != nil {
		_ = w.conn.Close()
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

func (w *WSConn) Write(msgType, msg string) {
	err := w.conn.WriteMessage(1, []byte(msgType+` {"data":"`+msg+`"}`))
	if err != nil {
		logAndDelay(err)
		w.Reconnect()
	}
}

func (w *WSConn) WritePrivate(u string, msg string) {
	w.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	err := w.conn.WriteMessage(1, []byte(`PRIVMSG {"nick":"`+u+`", data":"`+msg+`"}`))
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
	log.Println("Reconnecting in 5 Seconds...")
	time.Sleep(5 * time.Second)
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	var chLog = make(chan Message, 10)
	var chHandle = make(chan Message, 2)

	go WriteFile(chLog)

	ws := &WSConn{
		dialer: websocket.Dialer{HandshakeTimeout: 20 * time.Second},
		heads:  http.Header{"Origin": origin, "Cookie": authToken},
	}
	log.Println("Connecting to", url)
	ws.Reconnect()
	log.Println("Connected to", url)

	go HandleMessage(ws, chHandle)

	for {
		msg := ws.Read()
		// log.Println(msg)

		chLog <- msg
		chHandle <- msg
	}
}
