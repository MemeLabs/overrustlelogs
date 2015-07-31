package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	s "strings"
	"time"
)

func WriteFile(ch <-chan Message) {

	for {
		res := <-ch
		basePath := "public/_public/Destinygg chatlog/"
		timestamp := time.Unix(res.Timestamp/1000, 0).UTC()
		msgDate := timestamp.Format("2006-01-02")
		msgTime := timestamp.Format("[2006-01-02 15:04:05 MST] ")
		monthYear := timestamp.Format("January 2006")
		currMonthYear := time.Now().Format("January 2006")

		checkFolders(basePath + currMonthYear + "/userlogs/")
		go writePremiumFile(res.Nick, res.Data, msgTime, currMonthYear)

		switch res.Command {
		case "BAN":
			f, err := OpenFile(fmt.Sprintf("%s%s/bans.txt", basePath, monthYear))
			if err != nil {
				LogErr(err)
				continue
			}
			f.WriteString(msgTime + res.Data + " banned by " + res.Nick + "\n")
			f.Close()
			log.Printf("%s > %s banned %s", res.Command, res.Nick, res.Data)
		case "UNBAN":
			f, err := OpenFile(fmt.Sprintf("%s%s/bans.txt", basePath, monthYear))
			if err != nil {
				LogErr(err)
				continue
			}
			f.WriteString(msgTime + res.Data + " unbanned by " + res.Nick + "\n")
			f.Close()
			log.Printf("%s > %s unbanned %s", res.Command, res.Nick, res.Data)
		case "MUTE":
			f, err := OpenFile(fmt.Sprintf("%s%s/bans.txt", basePath, monthYear))
			if err != nil {
				LogErr(err)
				continue
			}
			f.WriteString(msgTime + res.Data + " muted by " + res.Nick + "\n")
			f.Close()
			log.Printf("%s > %s muted %s", res.Command, res.Nick, res.Data)
		case "UNMUTE":
			f, err := OpenFile(fmt.Sprintf("%s%s/bans.txt", basePath, monthYear))
			if err != nil {
				LogErr(err)
				continue
			}
			f.WriteString(msgTime + res.Data + " unmuted by " + res.Nick + "\n")
			f.Close()
			log.Printf("%s > %s unmuted %s", res.Command, res.Nick, res.Data)
		case "BROADCAST":
			if s.Contains(res.Data, "subscriber!") || s.Contains(res.Data, "resubscribed on Twitch!") {
				f, err := OpenFile(fmt.Sprintf("%s%s/subs.txt", basePath, monthYear))
				if err != nil {
					LogErr(err)
					continue
				}
				f.WriteString(msgTime + res.Data + "\n")
				f.Close()
			}
		case "MSG":
			f, err := OpenFile(fmt.Sprintf("%s%s/%s.txt", basePath, monthYear, msgDate))
			if err != nil {
				LogErr(err)
				continue
			}
			f.WriteString(msgTime + res.Nick + ": " + res.Data + "\n")
			f.Close()
			f, err = OpenFile(fmt.Sprintf("%s%s/userlogs/%s.txt", basePath, monthYear, res.Nick))
			if err != nil {
				LogErr(err)
				continue
			}
			f.WriteString(msgTime + res.Nick + ": " + res.Data + "\n")
			f.Close()
			log.Printf("%s > %s: %s", res.Command, res.Nick, res.Data)
		}
	}
}

func LogErr(err error) {
	if err != nil {
		log.Printf("ERROR: %s\n", err)
	}
}

func OpenFile(p string) (*os.File, error) {
	return os.OpenFile(fmt.Sprintf(p),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0755)
}

func checkFolders(p string) {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		LogErr(err)
		return
	}

	p = filepath.Join(dir, p)
	_, err = os.Stat(p)
	if err != nil {
		createFolders(p)
		return
	}
}

func createFolders(p string) {
	err := os.MkdirAll(p, 0755)
	if err != nil {
		LogErr(err)
		return
	}

}

func writePremiumFile(nick, msg, time, date string) {
	p, err := ioutil.ReadFile("premium.txt")
	if err != nil {
		LogErr(err)
		return
	}

	namelist := s.Split(string(p), ",")

	for _, n := range namelist {
		if s.Contains(s.ToLower(msg), s.ToLower(n)) {
			checkFolders("public/_public/premium/" + s.ToLower(n))

			f, err := OpenFile("public/_public/premium/" + s.ToLower(n) + "/" + date + ".txt")
			if err != nil {
				LogErr(err)
				return
			}
			defer f.Close()

			f.WriteString(time + nick + ": " + msg + "\n")
		}
	}
}
