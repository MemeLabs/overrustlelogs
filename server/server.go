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
	"path/filepath"
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

var cache *logCache

func init() {
	configPath := flag.String("config", "", "config path")
	flag.Parse()
	common.SetupConfig(*configPath)
}

// Start server
func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	cache = newLogCache()

	r := mux.NewRouter()
	r.HandleFunc("/", BaseHandle).Methods("GET")
	r.HandleFunc("/contact", ContactHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}", ChannelHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}", MonthHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/{date:[0-9]{4}-[0-9]{2}-[0-9]{2}}.txt", DayHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/userlogs", UsersHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/userlogs/{nick:[a-zA-Z0-9_-]{1,25}}.txt", UserHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/premium/{nick:[a-zA-Z0-9_-]{1,25}}", PremiumHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/premium/{nick:[a-zA-Z0-9_-]{1,25}}/{month:[a-zA-Z]+ [0-9]{4}}.txt", PremiumUserHandle).Methods("GET")
	r.HandleFunc("/Destinygg chatlog/{month:[a-zA-Z]+ [0-9]{4}}/broadcaster.txt", DestinyBroadcasterHandle).Methods("GET")
	r.HandleFunc("/Destinygg chatlog/{month:[a-zA-Z]+ [0-9]{4}}/subscribers.txt", DestinySubscriberHandle).Methods("GET")
	r.HandleFunc("/Destinygg chatlog/{month:[a-zA-Z]+ [0-9]{4}}/bans.txt", DestinyBanHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/broadcaster.txt", BroadcasterHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/subscribers.txt", SubscriberHandle).Methods("GET")
	r.HandleFunc("/api/v1/stalk/{channel:[a-zA-Z0-9_-]+ chatlog}/{nick:[a-zA-Z0-9_-]+}.json", StalkHandle).Queries("limit", "{limit:[0-9]+}").Methods("GET")
	r.NotFoundHandler = http.HandlerFunc(NotFoundHandle)
	go http.ListenAndServe(common.GetConfig().Server.Address, r)

	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
	select {
	case <-sigint:
		log.Println("i love you guys, be careful")
		os.Exit(0)
	}
}

// NotFoundHandle channel index
func NotFoundHandle(w http.ResponseWriter, r *http.Request) {
	serveError(w, ErrNotFound)
}

// BaseHandle channel index
func BaseHandle(w http.ResponseWriter, r *http.Request) {
	paths, err := readDirIndex(common.GetConfig().LogPath)
	if err != nil {
		serveError(w, err)
		return
	}
	serveDirIndex(w, []string{}, paths)
}

// ContactHandle contact page
func ContactHandle(w http.ResponseWriter, r *http.Request) {
	tpl, err := ace.Load(common.GetConfig().Server.ViewPath+"/layout", common.GetConfig().Server.ViewPath+"/contact", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-type", "text/html")
	if err := tpl.Execute(w, nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// ChannelHandle channel index
func ChannelHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	paths, err := readDirIndex(common.GetConfig().LogPath + "/" + vars["channel"])
	if err != nil {
		serveError(w, err)
		return
	}
	sort.Sort(dirsByMonth(paths))
	serveDirIndex(w, []string{vars["channel"]}, paths)
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
	serveDirIndex(w, []string{vars["channel"], vars["month"]}, paths)
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
			common.ReadNickList(nicks, common.GetConfig().LogPath+"/"+vars["channel"]+"/"+vars["month"]+"/"+file.Name())
		}
	}
	names := make([]string, 0, len(nicks))
	for nick := range nicks {
		names = append(names, nick+".txt")
	}
	sort.Sort(handysort.Strings(names))
	serveDirIndex(w, []string{vars["channel"], vars["month"], "userlogs"}, names)
}

// UserHandle user log
func UserHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serveFilteredLogs(w, common.GetConfig().LogPath+"/"+vars["channel"]+"/"+vars["month"], nickFilter(vars["nick"]))
}

// PremiumHandle premium user log index
func PremiumHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	paths, err := readDirIndex(common.GetConfig().LogPath + "/" + vars["channel"])
	if err != nil {
		serveError(w, err)
		return
	}
	for i := range paths {
		paths[i] += ".txt"
	}
	serveDirIndex(w, []string{vars["channel"], "premium", vars["nick"]}, paths)
}

// PremiumUserHandle user logs + replies
func PremiumUserHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	nick := bytes.ToLower([]byte(vars["nick"]))
	filter := func(line []byte) bool {
		return bytes.Contains(bytes.ToLower(line), nick)
	}
	serveFilteredLogs(w, common.GetConfig().LogPath+"/"+vars["channel"]+"/"+vars["month"], filter)
}

// BroadcasterHandle channel index
func BroadcasterHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	nick := vars["channel"][:len(vars["channel"])-8]
	search, err := common.NewNickSearch(common.GetConfig().LogPath+"/"+vars["channel"], nick)
	if err != nil {
		serveError(w, ErrNotFound)
		return
	}
	rs, err := search.Next()
	if err == io.EOF {
		serveError(w, ErrNotFound)
		return
	} else if err != nil {
		serveError(w, err)
	}
	serveFilteredLogs(w, common.GetConfig().LogPath+"/"+vars["channel"]+"/"+vars["month"], nickFilter(rs.Nick()))
}

// SubscriberHandle channel index
func SubscriberHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serveFilteredLogs(w, common.GetConfig().LogPath+"/"+vars["channel"]+"/"+vars["month"], nickFilter("twitchnotify"))
}

// DestinyBroadcasterHandle destiny logs
func DestinyBroadcasterHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serveFilteredLogs(w, common.GetConfig().LogPath+"/Destinygg chatlog/"+vars["month"], nickFilter("Destiny"))
}

// DestinySubscriberHandle destiny subscriber logs
func DestinySubscriberHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serveFilteredLogs(w, common.GetConfig().LogPath+"/Destinygg chatlog/"+vars["month"], nickFilter("Subscriber"))
}

// DestinyBanHandle channel ban list
func DestinyBanHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serveFilteredLogs(w, common.GetConfig().LogPath+"/Destinygg chatlog/"+vars["month"], nickFilter("Ban"))
}

// StalkHandle return n most recent lines of chat for user
func StalkHandle(w http.ResponseWriter, r *http.Request) {
	type Error struct {
		Error string `json:"error"`
	}

	w.Header().Set("Content-type", "application/json")
	vars := mux.Vars(r)
	limit, err := strconv.ParseUint(vars["limit"], 10, 32)
	if err != nil {
		d, _ := json.Marshal(Error{err.Error()})
		http.Error(w, string(d), http.StatusBadRequest)
		return
	}
	if limit > uint64(common.GetConfig().Server.MaxStalkLines) {
		limit = uint64(common.GetConfig().Server.MaxStalkLines)
	} else if limit < 1 {
		limit = 1
	}
	buf := make([]string, limit)
	index := limit
	search, err := common.NewNickSearch(common.GetConfig().LogPath+"/"+vars["channel"], vars["nick"])
	if err != nil {
		d, _ := json.Marshal(Error{err.Error()})
		http.Error(w, string(d), http.StatusNotFound)
		return
	}

ScanLogs:
	for {
		rs, err := search.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			d, _ := json.Marshal(Error{err.Error()})
			http.Error(w, string(d), http.StatusInternalServerError)
			return
		}
		data, err := readLogFile(common.GetConfig().LogPath + "/" + vars["channel"] + "/" + rs.Month() + "/" + rs.Day())
		if err != nil {
			d, _ := json.Marshal(Error{err.Error()})
			http.Error(w, string(d), http.StatusInternalServerError)
			return
		}
		lines := [][]byte{}
		r := bufio.NewReaderSize(bytes.NewReader(data), len(data))
		filter := nickFilter(rs.Nick())
		for {
			line, err := r.ReadSlice('\n')
			if err != nil {
				if err != io.EOF {
					log.Printf("error reading bytes %s", err)
				}
				break
			}
			if filter(line) {
				lines = append(lines, line[0:len(line)-1])
			}
		}
		for i := len(lines) - 1; i >= 0; i-- {
			index--
			buf[index] = string(lines[i])
			if index == 0 {
				break ScanLogs
			}
		}
	}

	if index == limit {
		d, _ := json.Marshal(Error{"User not found"})
		http.Error(w, string(d), http.StatusInternalServerError)
		return
	}
	type Line struct {
		Timestamp int64  `json:"timestamp"`
		Text      string `json:"text"`
	}
	data := struct {
		Nick  string `json:"nick"`
		Lines []Line `json:"lines"`
	}{
		Lines: []Line{},
	}
	for i := int(index); i < len(buf); i++ {
		t, err := time.Parse("2006-01-02 15:04:05 MST", buf[i][1:24])
		if err != nil {
			continue
		}
		ci := strings.Index(buf[i][LogLinePrefixLength:], ":")
		data.Nick = buf[i][LogLinePrefixLength : LogLinePrefixLength+ci]
		data.Lines = append(data.Lines, Line{
			Timestamp: t.Unix(),
			Text:      buf[i][ci+LogLinePrefixLength+2:],
		})
	}
	d, _ := json.Marshal(data)
	w.Write(d)
}

type dirsByMonth []string

func (l dirsByMonth) Len() int {
	return len(l)
}

func (l dirsByMonth) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func (l dirsByMonth) Less(i, j int) bool {
	format := "January 2006"
	a, err := time.Parse(format, l[i])
	if err != nil {
		return true
	}
	b, err := time.Parse(format, l[j])
	if err != nil {
		return false
	}
	return b.After(a)
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

func nickFilter(nick string) func([]byte) bool {
	nick += ":"
	return func(line []byte) bool {
		for i := 0; i < len(nick); i++ {
			if i+LogLinePrefixLength > len(line) || line[i+LogLinePrefixLength] != nick[i] {
				return false
			}
		}
		return true
	}
}

// serveError ...
func serveError(w http.ResponseWriter, e error) {
	tpl, err := ace.Load(common.GetConfig().Server.ViewPath+"/layout", common.GetConfig().Server.ViewPath+"/error", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := map[string]interface{}{}
	if e == ErrNotFound {
		w.WriteHeader(http.StatusNotFound)
		data["Message"] = e.Error()
	} else if e != nil {
		// w.WriteHeader(http.StatusInternalServerError)
		data["Message"] = e.Error()
	} else {
		// w.WriteHeader(http.StatusInternalServerError)
		data["Message"] = "Unknown Error"
	}
	w.Header().Set("Content-type", "text/html")
	if err := tpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// serveDirIndex ...
func serveDirIndex(w http.ResponseWriter, base []string, paths []string) {
	tpl, err := ace.Load(common.GetConfig().Server.ViewPath+"/layout", common.GetConfig().Server.ViewPath+"/directory", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := map[string]interface{}{
		"Breadcrumbs": []map[string]string{},
		"Paths":       []map[string]string{},
	}
	basePath := ""
	for _, path := range base {
		basePath += "/" + path
		data["Breadcrumbs"] = append(data["Breadcrumbs"].([]map[string]string), map[string]string{
			"Path": basePath,
			"Name": path,
		})
	}
	basePath += "/"
	for _, path := range paths {
		icon := "file"
		if filepath.Ext(path) == "" {
			icon = "folder-close"
		}
		data["Paths"] = append(data["Paths"].([]map[string]string), map[string]string{
			"Path": basePath + path,
			"Name": path,
			"Icon": icon,
		})
	}
	w.Header().Set("Content-type", "text/html")
	if err := tpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func serveFilteredLogs(w http.ResponseWriter, path string, filter func([]byte) bool) {
	logs, err := readLogDir(path)
	if err != nil {
		serveError(w, err)
		return
	}
	w.Header().Set("Content-type", "text/plain")
	start := time.Now()
	today := time.Now().UTC().Format("2006-01-02") + ".txt"
	for _, name := range logs {
		var lines [][]byte
		fullPath := path + "/" + name
		if name != today {
			if l, ok := cache.get(fullPath); ok {
				lines = l
			}
		}
		if len(lines) == 0 {
			data, err := readLogFile(fullPath)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			lines = bytes.Split(data, []byte{'\n'})
			if name != today {
				cache.add(fullPath, lines, int64(len(data)))
			}
		}
		for i := 0; i < len(lines); i++ {
			if err != nil {
				if err != io.EOF {
					log.Printf("error reading bytes %s", err)
				}
				break
			}
			if filter(lines[i]) {
				w.Write(lines[i])
				w.Write([]byte{'\n'})
			}
		}
	}
	log.Printf("generated %s user log in %s", path, time.Since(start))
}
