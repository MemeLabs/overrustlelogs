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
		filePath := "public/_public/Destinygg chatlog/"
		timestamp := time.Unix(res.Timestamp/1000, 0).UTC()
		currDate := timestamp.Format("01-02-2006")
		currTime := timestamp.Format("[Jan 02 2006 15:04:05 MST] ")
		monthYear := timestamp.Format("January 2006")

		checkFolders(monthYear)
		go writePremiumFile(res.Nick, res.Data, currTime, monthYear)

		switch res.Command {
		case "BROADCAST":
			f, err := openFile(fmt.Sprintf("%s%s/subs.txt", filePath, monthYear))
			if err != nil {
				LogErr(err)
				continue
			}
			f.WriteString(currTime + res.Data + "\n")
			f.Close()
		case "BAN":
			f, err := openFile(fmt.Sprintf("%s%s/bans.txt", filePath, monthYear))
			if err != nil {
				LogErr(err)
				continue
			}
			f.WriteString(currTime + res.Data + " banned by " + res.Nick + "\n")
			f.Close()
			log.Printf("%s > %s banned %s", res.Command, res.Nick, res.Data)
		case "UNBAN":
			f, err := openFile(fmt.Sprintf("%s%s/bans.txt", filePath, monthYear))
			if err != nil {
				LogErr(err)
				continue
			}
			f.WriteString(currTime + res.Data + " unbanned by " + res.Nick + "\n")
			f.Close()
			log.Printf("%s > %s unbanned %s", res.Command, res.Nick, res.Data)
		case "MUTE":
			f, err := openFile(fmt.Sprintf("%s%s/bans.txt", filePath, monthYear))
			if err != nil {
				LogErr(err)
				continue
			}
			f.WriteString(currTime + res.Data + " muted by " + res.Nick + "\n")
			f.Close()
			log.Printf("%s > %s muted %s", res.Command, res.Nick, res.Data)
		case "UNMUTE":
			f, err := openFile(fmt.Sprintf("%s%s/bans.txt", filePath, monthYear))
			if err != nil {
				LogErr(err)
				continue
			}
			f.WriteString(currTime + res.Data + " unmuted by " + res.Nick + "\n")
			f.Close()
			log.Printf("%s > %s unmuted %s", res.Command, res.Nick, res.Data)
		case "MSG":
			f, err := openFile(fmt.Sprintf("%s%s/%s.txt", filePath, monthYear, currDate))
			if err != nil {
				LogErr(err)
				continue
			}
			f.WriteString(currTime + res.Nick + ": " + res.Data + "\n")
			f.Close()
			f, err = openFile(fmt.Sprintf("%s%s/userlogs/%s.txt", filePath, monthYear, res.Nick))
			if err != nil {
				LogErr(err)
				continue
			}
			f.WriteString(currTime + res.Nick + ": " + res.Data + "\n")
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

func openFile(p string) (*os.File, error) {
	f, err := os.OpenFile(fmt.Sprintf(p),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0755)
	if err != nil {
		LogErr(err)
		return nil, err
	}
	return f, nil
}

func checkFolders(d string) {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		LogErr(err)
		return
	}

	p := filepath.Join(dir, "public/_public/Destinygg chatlog/"+d+"/userlogs/")
	_, err = os.Stat(p)
	if err != nil {
		createFolders("Destinygg chatlog/" + d + "/userlogs/")
		return
	}
}

func createFolders(p string) {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		LogErr(err)
		return
	}

	p = filepath.Join(dir, "public/_public/"+p)
	_, err = os.Stat(p)
	if err != nil {
		err = os.MkdirAll(p, 0755)
		if err != nil {
			LogErr(err)
			return
		}
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
			createFolders("premium/" + s.ToLower(n))

			f, err := openFile("public/_public/premium/" + s.ToLower(n) + "/" + date + ".txt")
			if err != nil {
				LogErr(err)
				return
			}

			f.WriteString(time + nick + ": " + msg + "\n")
			f.Close()
		}
	}
}
