package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/slugalisk/overrustlelogs/common"
	"github.com/xlab/handysort"
	"github.com/yosssi/ace"
)

// temp ish.. move to config
const (
	LogLinePrefixLength = 26
)

// errors
var (
	ErrNotFound = errors.New("file not found")
)

// log file extension pattern
var (
	LogExtension   = regexp.MustCompile("\\.txt(\\.lz4)?$")
	NicksExtension = regexp.MustCompile("\\.nicks\\.lz4$")
)

func init() {
	configPath := flag.String("config", "", "config path")
	flag.Parse()
	common.SetupConfig(*configPath)
}

// Start server
func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	r := mux.NewRouter()

	r.HandleFunc("/", BaseHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}", ChannelHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}", MonthHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/{date:[0-9]{4}-[0-9]{2}-[0-9]{2}}.txt", DayHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/userlogs", UsersHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/userlogs/{user:[a-zA-Z0-9_-]{1,25}}.txt", UserHandle).Methods("GET")
	r.HandleFunc("/Destinygg chatlog/{month:[a-zA-Z]+ [0-9]{4}}/broadcaster.txt", DestinyBroadcasterHandle).Methods("GET")
	r.HandleFunc("/Destinygg chatlog/{month:[a-zA-Z]+ [0-9]{4}}/subscribers.txt", DestinySubscriberHandle).Methods("GET")
	r.HandleFunc("/Destinygg chatlog/{month:[a-zA-Z]+ [0-9]{4}}/bans.txt", DestinyBanHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/broadcaster.txt", BroadcasterHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/subscribers.txt", SubscriberHandle).Methods("GET")
	r.HandleFunc("/api/v1/stalk/{channel:[a-zA-Z0-9_-]+ chatlog}/{nick:[a-zA-Z0-9_-]+}.json", StalkHandle).Queries("limit", "{limit:[0-9]+}").Methods("GET")

	go http.ListenAndServe(common.GetConfig().Server.Address, r)

	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
	select {
	case <-sigint:
		log.Println("i love you guys, be careful")
		os.Exit(1)
	}
}

// BaseHandle channel index
func BaseHandle(w http.ResponseWriter, r *http.Request) {
	paths, err := readDirIndex(common.GetConfig().LogPath)
	if err != nil {
		serveError(w, err)
		return
	}
	serveDirIndex(w, "/", paths)
}

// ChannelHandle channel index
func ChannelHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	paths, err := readDirIndex(common.GetConfig().LogPath + "/" + vars["channel"])
	if err != nil {
		serveError(w, err)
		return
	}
	serveDirIndex(w, "/"+vars["channel"]+"/", paths)
}

// MonthHandle channel index
func MonthHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	paths, err := readLogDir(common.GetConfig().LogPath + "/" + vars["channel"] + "/" + vars["month"])
	if err != nil {
		serveError(w, err)
		return
	}
	metaPaths := []string{"userlogs", "broadcaster.txt", "subscribers.txt"}
	if vars["channel"] == "Destinygg chatlog" {
		metaPaths = append(metaPaths, "bans.txt")
	}
	paths = append(paths, metaPaths...)
	copy(paths[len(metaPaths):], paths)
	copy(paths, metaPaths)
	for i, path := range paths {
		paths[i] = LogExtension.ReplaceAllString(path, ".txt")
	}
	serveDirIndex(w, "/"+vars["channel"]+"/"+vars["month"]+"/", paths)
}

// DayHandle channel index
func DayHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	data, err := readLogFile(common.GetConfig().LogPath + "/" + vars["channel"] + "/" + vars["month"] + "/" + vars["date"])
	if err != nil {
		serveError(w, err)
		return
	}
	w.Header().Set("Content-type", "text/plain")
	w.Write(data)
}

// UsersHandle channel index
func UsersHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	f, err := os.Open(common.GetConfig().LogPath + "/" + vars["channel"] + "/" + vars["month"])
	if err != nil {
		serveError(w, ErrNotFound)
		return
	}
	files, err := f.Readdir(0)
	if err != nil {
		serveError(w, err)
		return
	}
	nicks := common.NickList{}
	for _, file := range files {
		if NicksExtension.MatchString(file.Name()) {
			nicks.ReadFrom(common.GetConfig().LogPath + "/" + vars["channel"] + "/" + vars["month"] + "/" + file.Name())
		}
	}
	names := make([]string, 0, len(nicks))
	for nick := range nicks {
		names = append(names, nick+".txt")
	}
	sort.Sort(handysort.Strings(names))
	serveDirIndex(w, "/"+vars["channel"]+"/"+vars["month"]+"/userlogs/", names)
}

// UserHandle channel index
func UserHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serveUserLog(w, common.GetConfig().LogPath+"/"+vars["channel"]+"/"+vars["month"], vars["user"])
}

// BroadcasterHandle channel index
func BroadcasterHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serveUserLog(w, common.GetConfig().LogPath+"/"+vars["channel"]+"/"+vars["month"], vars["channel"][:len(vars["channel"])-8])
}

// SubscriberHandle channel index
func SubscriberHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serveUserLog(w, common.GetConfig().LogPath+"/Destinygg chatlog/"+vars["month"], "twitchnotify")
}

// DestinyBroadcasterHandle destiny logs
func DestinyBroadcasterHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serveUserLog(w, common.GetConfig().LogPath+"/Destinygg chatlog/"+vars["month"], "Destiny")
}

// DestinySubscriberHandle destiny subscriber logs
func DestinySubscriberHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serveUserLog(w, common.GetConfig().LogPath+"/Destinygg chatlog/"+vars["month"], "Subscriber")
}

// DestinyBanHandle channel ban list
func DestinyBanHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serveUserLog(w, common.GetConfig().LogPath+"/Destinygg chatlog/"+vars["month"], "Ban")
}

// StalkHandle return n most recent lines of chat for user
func StalkHandle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-type", "application/json")

	type Error struct {
		Error string `json:"error"`
	}

	vars := mux.Vars(r)
	nick := strings.ToLower(vars["nick"])
	prefix := vars["nick"] + ":"
	date := time.Now()
	limit, err := int(strconv.ParseInt(vars["limit"], 10, 32))
	if err != nil {
		d, _ := json.Marshal(Error{err.Error()})
		http.Error(w, string(d), http.StatusInternalServerError)
		return
	}
	if limit > common.GetConfig().Server.MaxStalkLines {
		limit = common.GetConfig().Server.MaxStalkLines
	} else if limit < 1 {
		limit = 1
	}
	buf := make([]string, limit)
	index := limit
	f, err := os.Open(common.GetConfig().LogPath + "/" + vars["channel"])
	if err != nil {
		d, _ := json.Marshal(Error{err.Error()})
		http.Error(w, string(d), http.StatusInternalServerError)
		return
	}
	names, err := f.Readdirnames(0)
	if err != nil {
		d, _ := json.Marshal(Error{err.Error()})
		http.Error(w, string(d), http.StatusInternalServerError)
		return
	}
	months := make(map[string]struct{}, len(names))
	for _, name := range names {
		months[name] = struct{}{}
	}

ScanLogs:
	for {
		if _, ok := months[date.Format("January 2006")]; !ok {
			log.Println("missing", date.Format("January 2006"))
			break ScanLogs
		}
		nicks := common.NickList{}
		nicks.ReadFrom(common.GetConfig().LogPath+"/"+vars["channel"]+date.Format("/January 2006/2006-01-02")+".nicks", strings.ToLower)
		if _, ok := nicks[nick]; ok {
			data, err := readLogFile(common.GetConfig().LogPath + "/" + vars["channel"] + date.Format("/January 2006/2006-01-02"))
			if err != nil {
				d, _ := json.Marshal(Error{err.Error()})
				http.Error(w, string(d), http.StatusInternalServerError)
				return
			}
			lines := [][]byte{}
			r := bufio.NewReaderSize(bytes.NewReader(data), len(data))
		ReadLine:
			for {
				line, err := r.ReadSlice('\n')
				if err != nil {
					if err != io.EOF {
						log.Printf("error reading bytes %s", err)
					}
					break
				}
				if LogLinePrefixLength+len(prefix) > len(line) || !bytes.EqualFold([]byte(prefix), line[LogLinePrefixLength:LogLinePrefixLength+len(prefix)]) {
					continue ReadLine
				}
				lines = append(lines, line[0:len(line)-1])
			}
			for i := len(lines) - 1; i >= 0; i-- {
				index--
				buf[index] = string(lines[i])
				if index == 0 {
					break ScanLogs
				}
			}
		}
		date = date.Add(-24 * time.Hour)
	}

	if index == limit {
		d, _ := json.Marshal(Error{"User not found"})
		http.Error(w, string(d), http.StatusInternalServerError)
		return
	}

	d, _ := json.Marshal(struct {
		Lines []string `json:"lines"`
	}{buf[index:]})
	w.Write(d)
}

func readDirIndex(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, ErrNotFound
	}
	names, err := f.Readdirnames(0)
	if err != nil {
		return nil, err
	}
	sort.Sort(handysort.Strings(names))
	return names, nil
}

func readLogDir(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, ErrNotFound
	}
	files, err := f.Readdir(0)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, file := range files {
		if LogExtension.MatchString(file.Name()) {
			names = append(names, file.Name())
		}
	}
	sort.Sort(handysort.Strings(names))
	return names, nil
}

func readLogFile(path string) ([]byte, error) {
	var buf []byte
	path = LogExtension.ReplaceAllString(path, "")
	buf, err := common.ReadCompressedFile(path + ".txt")
	if os.IsNotExist(err) {
		f, err := os.Open(path + ".txt")
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		buf, err = ioutil.ReadAll(f)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	return buf, nil
}

// serveError ...
func serveError(w http.ResponseWriter, err error) {
	_, ok := w.Header()["Content-Type"]

	if !ok {
		w.Header().Set("Content-type", "text/plain")
	}
	if err == ErrNotFound {
		http.Error(w, err.Error(), http.StatusNotFound)
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		http.Error(w, "Unknown Error", http.StatusInternalServerError)
	}
}

// serveDirIndex ...
func serveDirIndex(w http.ResponseWriter, base string, paths []string) {
	tpl, err := ace.Load(common.GetConfig().Server.ViewPath+"/layout", common.GetConfig().Server.ViewPath+"/directory", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := map[string]interface{}{
		"Title": base,
		"Paths": []map[string]string{},
	}
	for _, path := range paths {
		data["Paths"] = append(data["Paths"].([]map[string]string), map[string]string{
			"ClassName": "directory",
			"Path":      base + path,
			"Name":      path,
		})
	}
	if err := tpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func serveUserLog(w http.ResponseWriter, path string, user string) {
	user += ":"
	filter := func(line []byte) bool {
		for i := 0; i < len(user); i++ {
			if i+LogLinePrefixLength > len(line) || line[i+LogLinePrefixLength] != user[i] {
				return false
			}
		}
		return true
	}
	serveFilteredLogs(w, path, filter)
}

func serveFilteredLogs(w http.ResponseWriter, path string, filter func([]byte) bool) {
	logs, err := readLogDir(path)
	if err != nil {
		serveError(w, err)
		return
	}
	w.Header().Set("Content-type", "text/plain")
	for _, name := range logs {
		data, err := readLogFile(path + "/" + name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		r := bufio.NewReaderSize(bytes.NewReader(data), len(data))
		for {
			line, err := r.ReadSlice('\n')
			if err != nil {
				if err != io.EOF {
					log.Printf("error reading bytes %s", err)
				}
				break
			}
			if filter(line) {
				w.Write(line)
			}
		}
	}
}
