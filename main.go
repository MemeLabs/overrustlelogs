package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

//Response struct.
type Response struct {
	Nick      string   `json:"nick"`
	NickLower string   `json:"nicklower"`
	Features  []string `json:"features"`
	Data      string   `json:"data"`
	Timestamp int64    `json:"timestamp"`
	Command   string   `json:"command"`
}

type UserData struct {
	Nick    string
	File    *os.File
	LastMSG time.Time
}

// WriteFile uses the returned struct from parse to log it into a file.
func WriteFile(ch chan Response) {
	var userdata []UserData

	go func() {
		for {
			userdata = closeFile(userdata)
			time.Sleep(10 * time.Second)
		}
	}()

	for {
		message := <-ch
		filePath := "public/_public/Destinygg chatlog/"
		messageTime := time.Unix(message.Timestamp/1000, 0).UTC()
		currDate := messageTime.Format("01-02-2006")
		currTime := messageTime.Format("[Jan 02 2006 15:04:05 MST] ")
		monthYear := messageTime.Format("January 2006")

		CheckFolders(monthYear)

		if message.Command == "BROADCAST" {
			f, err := openFile(fmt.Sprintf("%s%s/subs.txt", filePath, monthYear))
			if err != nil {
				log.Println(err)
			}
			f.WriteString(currTime + message.Data + "\n")
			f.Close()
		}
		if message.Command == "BAN" {
			f, err := openFile(fmt.Sprintf("%s%s/bans.txt", filePath, monthYear))
			if err != nil {
				log.Println(err)
			}
			f.WriteString(currTime + message.Data + " banned by " + message.Nick + "\n")
			f.Close()
		}
		if message.Command == "UNBAN" {
			f, err := openFile(fmt.Sprintf("%s%s/bans.txt", filePath, monthYear))
			if err != nil {
				log.Println(err)
			}
			f.WriteString(currTime + message.Data + " unbanned by " + message.Nick + "\n")
			f.Close()
		}
		if message.Command == "MUTE" {
			f, err := openFile(fmt.Sprintf("%s%s/bans.txt", filePath, monthYear))
			if err != nil {
				log.Println(err)
			}
			f.WriteString(currTime + message.Data + " muted by " + message.Nick + "\n")
			f.Close()
		}
		if message.Command == "UNMUTE" {
			f, err := openFile(fmt.Sprintf("%s%s/bans.txt", filePath, monthYear))
			if err != nil {
				log.Println(err)
			}
			f.WriteString(currTime + message.Data + " unmuted by " + message.Nick + "\n")
			f.Close()
		}
		if message.Command == "MSG" {
			f, err := openFile(fmt.Sprintf("%s%s/%s.txt", filePath, monthYear, currDate))
			if err != nil {
				log.Println("Error writing mainlog", err)
			}
			f.WriteString(currTime + message.Nick + ": " + message.Data + "\n")

			err = message.write(userdata, currTime)
			if err != nil {
				f, err := openFile(fmt.Sprintf("%s%s/userlogs/%s.txt", filePath, monthYear, message.Nick))
				if err != nil {
					log.Println("Error writing userlog", err)
				}
				f.WriteString(currTime + message.Nick + ": " + message.Data + "\n")
				userdata = append(userdata, UserData{message.Nick, f, time.Now()})
			}

		}
	}
}

func openFile(p string) (*os.File, error) {
	f, err := os.OpenFile(fmt.Sprintf(p),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0755)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func closeFile(ud []UserData) []UserData {
	if len(ud) > 0 {
		// log.Println("closing files startet-------------------------")
		for i, d := range ud {
			diff := time.Since(d.LastMSG)
			if diff.Seconds() > 59.0 {
				d.File.Close()
				// log.Println(d.Nick, "closed")
				ud = append(ud[:i], ud[i+1:]...)
			}
		}
	}
	return ud
}

func (r Response) write(ch []UserData, t string) error {
	for _, res := range ch {
		if res.Nick == r.Nick {
			res.File.WriteString(t + r.Nick + ": " + r.Data + "\n")
			res.update()
			return nil
		}
	}
	return errors.New("nope")
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
	r.NickLower = strings.ToLower(r.Nick)
	return r
}

// CheckFolders checks if the folders exist if not -> create them.
func CheckFolders(d string) {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Println(err)
	}
	p := filepath.Join(dir, "public/_public/Destinygg chatlog/"+d+"/userlogs")
	_, err = os.Stat(p)
	if err != nil {
		err = os.MkdirAll(p, 0755)
		if err != nil {
			log.Println(err)
		}
	}
}

func (u UserData) update() {
	u.LastMSG = time.Now()
}

func main() {
	var (
		msg     = make([]byte, 1024)
		resChan = make(chan Response, 10)
		addr    = "ws://destiny.gg:9998/ws"
		dialer  = websocket.Dialer{HandshakeTimeout: 5 * time.Second}
		headers = http.Header{"Origin": []string{"http://destiny.gg/"}}
	)

	go WriteFile(resChan)

	log.Println("Connecting to", addr)
again:
	ws, _, err := dialer.Dial(addr, headers)
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
			log.Printf("Reconnecting to server in %d seconds\n", 5)
			goto again
		}

		// log.Println(string(msg))

		res := Response{}
		res = Parse(msg, res)
		if res.Command == "BAN" || res.Command == "UNBAN" {
			log.Printf("%s > %s un/banned %s", res.Command, res.Nick, res.Data)
			resChan <- res
		}
		if res.Command == "MUTE" || res.Command == "UNMUTE" {
			log.Printf("%s > %s un/muted %s", res.Command, res.Nick, res.Data)
			resChan <- res
		}
		if res.Command == "MSG" {
			log.Printf("%s > %s: %s", res.Command, res.Nick, res.Data)
			resChan <- res
		}
		if res.Command == "BROADCAST" {
			if strings.Contains(res.Data, "subscriber!") {
				log.Printf("%s > %s", res.Command, res.Data)
				resChan <- res
			}
			if strings.Contains(res.Data, "resubscribed on Twitch!") {
				log.Printf("%s > %s", res.Command, res.Data)
				resChan <- res
			}
		}
	}
}
