package main

import (
	"log"
	"os"
	"path/filepath"
	s "strings"
	"time"
)

var (
	basePath = "public/_public/Destinygg chatlog/"
	baseURL  = "http://overrustlelogs.net/Destinygg%20chatlog/"
)

func HandleMessage(ws *WSConn, ch <-chan Message) {
	lastMSG := time.Now()
	for {
		m := <-ch
		rawSplitData := s.Split(m.Data, " ")
		data := s.ToLower(m.Data)
		splitData := s.Split(data, " ")
		switch m.Command {
		case "MSG":
			if s.Index(data, "!log") == 0 && time.Since(lastMSG) > 19 && len(splitData) == 1 {
				ws.Write("MSG", baseURL+s.Replace(time.Now().Format("January 2006"), " ", "%20", -1)+"/")
				lastMSG = time.Now()
				continue
			}
			if s.Index(data, "!log") == 0 && time.Since(lastMSG) > 19 {
				postLogs(rawSplitData[1], ws)
				lastMSG = time.Now()
				continue
			}
			if s.Index(data, "!log") == 0 && (m.Nick == "dbC_" || m.Nick == "Destiny" || m.Nick == "RightToBearArmsLOL" || m.Nick == "Tenseyi") {
				postLogs(rawSplitData[1], ws)
				lastMSG = time.Now()
				continue
			}
			if s.Index(data, "!del") == 0 && m.Nick == "dbC_" {
				deleteLogs(splitData[1], ws)
			}
		case "PRIVMSG":
			if s.Index(data, "!p") == 0 && m.Nick == "hephaestus" {
				ws.WritePrivate(m.Nick, "http://overrustlelogs.net/premium/hephaestus/"+s.Replace(time.Now().Format("January 2006"), " ", "%20", -1)+".txt")
			}
		}
	}

}

func postLogs(n string, ws *WSConn) {
	monthYear := time.Now().Format("January 2006")
	path := "public/_public/Destinygg chatlog/" + monthYear + "/userlogs/*"
	list, err := filepath.Glob(path)
	if err != nil {
		log.Printf("ERROR: %s\n", err)
		return
	}
	for _, p := range list {
		pNew := s.Replace(p, basePath+monthYear+"/userlogs/", "", -1)
		nNew := s.ToLower(n + ".txt")
		if s.ToLower(pNew) == nNew {
			ws.Write("MSG", baseURL+s.Replace(monthYear, " ", "%20", -1)+`/userlogs/`+pNew)
			return
		}
	}
	ws.Write("MSG", "No Logs found for "+n+"!")
}

func deleteLogs(n string, ws *WSConn) {
	monthYear := time.Now().Format("January 2006")
	path := basePath + monthYear + "/userlogs/" + n + ".txt"

	list, err := filepath.Glob(path)
	if err != nil {
		log.Printf("ERROR: %s\n", err)
		return
	}
	if len(list) == 0 {
		ws.Write("MSG", "No Logs found for "+n+"!")
		return
	}
	for _, d := range list {
		os.Remove(d)
	}
	ws.Write("MSG", "Deleted all Logs of "+n+"!")
}
