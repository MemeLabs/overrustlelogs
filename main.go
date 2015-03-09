package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"gopkg.in/mgo.v2"
)

//Response struct.
type Response struct {
	Nick      string   `json:"nick"`
	Features  []string `json:"features"`
	Data      string   `json:"data"`
	Timestamp int64    `json:"timestamp"`
	Command   string   `json:"command"`
}

// WriteFile uses the returned struct from parse to log it into a file.
func WriteFile(ch chan Response) {
	for {
		res := <-ch
		path := "public/_public/Destinygg chatlog/"
		t := time.Unix(res.Timestamp/1000, 0).UTC()
		currDate := t.Format("01-02-2006")
		cuurTime := t.Format("[Jan 02 2006 15:04:05 MST] ")
		monthYear := t.Format("January 2006")
		CheckFolders(monthYear)

		if res.Command == "BROADCAST" {
			sub, err := os.OpenFile(fmt.Sprintf("%s%s/subs.txt", path, monthYear),
				os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0755)
			if err != nil {
				log.Println(err)
			}
			sub.WriteString(cuurTime + res.Data + "\n")
			if len(ch) == 0 {
				sub.Close()
			}
			continue
		}
		u, err := os.OpenFile(fmt.Sprintf("%s%s/userlogs/%s.txt", path, monthYear, res.Nick),
			os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0755)
		if err != nil {
			log.Println(err)
		}
		u.WriteString(cuurTime + res.Nick + ": " + res.Data + "\n")

		m, err := os.OpenFile(fmt.Sprintf("%s%s/%s.txt", path, monthYear, currDate),
			os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0755)
		if err != nil {
			log.Println(err)
		}
		m.WriteString(cuurTime + res.Nick + ": " + res.Data + "\n")

		if len(ch) == 0 {
			m.Close()
			u.Close()
		}
	}
}

// Parse the raw message and return the struct.
func Parse(m []byte, r Response) Response {
	s := strings.SplitAfterN(string(m), " ", 2)
	if len(s) != 2 {
		return Response{}
	}
	err := json.Unmarshal([]byte(s[1]), &r)
	if err != nil {
		return Response{}
	}
	r.Command = strings.Trim(s[0], " ")
	r.Data = strings.Replace(r.Data, "\n", "", -1)
	return r
}

// CheckFolders checks if the folders exist if not create them.
func CheckFolders(d string) {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}
	p := filepath.Join(dir, "public/_public/Destinygg chatlog/"+d+"/userlogs")
	_, err = os.Stat(p)
	if err != nil {
		err = os.MkdirAll(p, 755)
		if err != nil {
			log.Fatal(err)
		}
	}
}

// InsertToDB not used yet.
func InsertToDB(s *mgo.Session, ch chan Response) {
	for {
		res := <-ch
		s.SetMode(mgo.Monotonic, true)
		timestamp := time.Unix(res.Timestamp/1000, 0).UTC()
		monthYear := timestamp.Format("January 2006")
		s := s.DB("Destinygg")

		err := s.C(monthYear).Insert(res)
		if err != nil {
			log.Println(err)
		}
	}
}

func main() {
	var (
		msg     = make([]byte, 1024)
		resChan = make(chan Response, 1)
		// dbChan  = make(chan Response, 1)
		url     = "ws://destiny.gg:9998/ws"
		delay   = 5
		dialer  = websocket.Dialer{HandshakeTimeout: 5 * time.Second}
		headers = http.Header{"Origin": []string{"http://destiny.gg/"}}
	)

	go WriteFile(resChan)
	// session, err := mgo.DialWithTimeout("localhost", time.Duration(1))
	// if err != nil {
	// 	log.Println(err)
	// }
	// go InsertToDB(session, dbChan)

	log.Println("Connecting to", url)
again:
	ws, _, err := dialer.Dial(url, headers)
	if err != nil {
		log.Println("Reonnecting in 5 seconds...")
		time.Sleep(5 * time.Second)
		goto again
	}
	defer ws.Close()

	log.Println("Connection Successful!")
	for {
		ws.SetReadDeadline(time.Now().Add(20 * time.Second))
		_, msg, err = ws.ReadMessage()
		if err != nil {
			ws.Close()
			if delay > 30 {
				delay = 5
			}
			log.Printf("Reconnecting to server in %v seconds\n", delay)
			time.Sleep(time.Duration(delay) * time.Second)
			delay += delay / 2
			goto again
		}

		res := Response{}
		res = Parse(msg, res)
		if res.Command == "MSG" {
			log.Printf("%s > %s: %s", res.Command, res.Nick, res.Data)
			resChan <- res
			// dbChan <- res
		}
		if res.Command == "BROADCAST" && strings.Contains(res.Data, "subscriber!") {
			log.Printf("%s > %s", res.Command, res.Data)
			resChan <- res
			// dbChan <- res
		}
	}
}
