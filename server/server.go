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
	"sync/atomic"
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
	ErrNotFound          = errors.New("file not found")
	ErrSearchKeyNotFound = errors.New("didn't find what you were looking for :(")
)

// log file extension pattern
var (
	LogExtension   = regexp.MustCompile(`\.txt(\.lz4)?$`)
	NicksExtension = regexp.MustCompile(`\.nicks\.lz4$`)
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

	d := NewDebugger()
	r := mux.NewRouter()
	r.HandleFunc("/", d.WatchHandle("Base", BaseHandle)).Methods("GET")
	r.HandleFunc("/contact", d.WatchHandle("Contact", ContactHandle)).Methods("GET")
	r.HandleFunc("/changelog", d.WatchHandle("Changelog", ChangelogHandle)).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}", d.WatchHandle("Channel", ChannelHandle)).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}", d.WatchHandle("Month", MonthHandle)).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/{date:[0-9]{4}-[0-9]{2}-[0-9]{2}}.txt", d.WatchHandle("Day", DayHandle)).Queries("search", "{filter:.+}").Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/{date:[0-9]{4}-[0-9]{2}-[0-9]{2}}.txt", d.WatchHandle("Day", DayHandle)).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/{date:[0-9]{4}-[0-9]{2}-[0-9]{2}}", d.WatchHandle("Day", DayHandle)).Queries("search", "{filter:.+}").Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/{date:[0-9]{4}-[0-9]{2}-[0-9]{2}}", d.WatchHandle("Day", DayHandle)).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/userlogs", d.WatchHandle("Users", UsersHandle)).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/userlogs/{nick:[a-zA-Z0-9_-]{1,25}}.txt", d.WatchHandle("User", UserHandle)).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/userlogs/{nick:[a-zA-Z0-9_-]{1,25}}", d.WatchHandle("User", UserHandle)).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/premium/{nick:[a-zA-Z0-9_-]{1,25}}", d.WatchHandle("Premium", PremiumHandle)).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/premium/{nick:[a-zA-Z0-9_-]{1,25}}/{month:[a-zA-Z]+ [0-9]{4}}.txt", d.WatchHandle("PremiumUser", PremiumUserHandle)).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/premium/{nick:[a-zA-Z0-9_-]{1,25}}/{month:[a-zA-Z]+ [0-9]{4}}", d.WatchHandle("PremiumUser", PremiumUserHandle)).Methods("GET")
	r.HandleFunc("/Destinygg chatlog/current", d.WatchHandle("DestinyBase", DestinyBaseHandle)).Methods("GET")
	r.HandleFunc("/Destinygg chatlog/current/{nick:[a-zA-Z0-9_]+}", d.WatchHandle("DestinyNick", DestinyNickHandle)).Queries("search", "{filter:.+}").Methods("GET")
	r.HandleFunc("/Destinygg chatlog/current/{nick:[a-zA-Z0-9_]+}.txt", d.WatchHandle("DestinyNick", DestinyNickHandle)).Methods("GET")
	r.HandleFunc("/Destinygg chatlog/current/{nick:[a-zA-Z0-9_]+}", d.WatchHandle("DestinyNick", DestinyNickHandle)).Methods("GET")
	r.HandleFunc("/Destinygg chatlog/{month:[a-zA-Z]+ [0-9]{4}}/broadcaster.txt", d.WatchHandle("DestinyBroadcaster", DestinyBroadcasterHandle)).Methods("GET")
	r.HandleFunc("/Destinygg chatlog/{month:[a-zA-Z]+ [0-9]{4}}/broadcaster", d.WatchHandle("DestinyBroadcaster", DestinyBroadcasterHandle)).Methods("GET")
	r.HandleFunc("/Destinygg chatlog/{month:[a-zA-Z]+ [0-9]{4}}/subscribers.txt", d.WatchHandle("DestinySubscriber", DestinySubscriberHandle)).Methods("GET")
	r.HandleFunc("/Destinygg chatlog/{month:[a-zA-Z]+ [0-9]{4}}/subscribers", d.WatchHandle("DestinySubscriber", DestinySubscriberHandle)).Methods("GET")
	r.HandleFunc("/Destinygg chatlog/{month:[a-zA-Z]+ [0-9]{4}}/bans.txt", d.WatchHandle("DestinyBan", DestinyBanHandle)).Methods("GET")
	r.HandleFunc("/Destinygg chatlog/{month:[a-zA-Z]+ [0-9]{4}}/bans", d.WatchHandle("DestinyBan", DestinyBanHandle)).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/broadcaster.txt", d.WatchHandle("Broadcaster", BroadcasterHandle)).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/broadcaster", d.WatchHandle("Broadcaster", BroadcasterHandle)).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/subscribers.txt", d.WatchHandle("Subscriber", SubscriberHandle)).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/subscribers", d.WatchHandle("Subscriber", SubscriberHandle)).Methods("GET")
	r.HandleFunc("/api/v1/stalk/{channel:[a-zA-Z0-9_-]+ chatlog}/{nick:[a-zA-Z0-9_-]+}.json", d.WatchHandle("Stalk", StalkHandle)).Queries("limit", "{limit:[0-9]+}").Methods("GET")
	r.HandleFunc("/api/v1/status.json", d.WatchHandle("Debug", d.HTTPHandle))
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

// Debugger logging...
type Debugger struct {
	counters map[string]*int64
}

// NewDebugger ...
func NewDebugger() *Debugger {
	d := &Debugger{make(map[string]*int64)}
	go func() {
		for {
			time.Sleep(time.Minute)
			d.DebugPrint()
		}
	}()
	return d
}

// WatchHandle ...
func (d *Debugger) WatchHandle(name string, f http.HandlerFunc) http.HandlerFunc {
	var c int64
	d.counters[name] = &c
	return func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&c, 1)
		f.ServeHTTP(w, r)
		atomic.AddInt64(&c, -1)
	}
}

func (d *Debugger) counts() map[string]int64 {
	counts := make(map[string]int64)
	for name, c := range d.counters {
		counts[name] = atomic.LoadInt64(c)
	}
	return counts
}

// DebugPrint ...
func (d *Debugger) DebugPrint() {
	log.Println(d.counts())
}

// HTTPHandle serve debugger status as JSON
func (d *Debugger) HTTPHandle(w http.ResponseWriter, r *http.Request) {
	b, _ := json.Marshal(d.counts())
	w.Write(b)
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

// ChangelogHandle changelog page
func ChangelogHandle(w http.ResponseWriter, r *http.Request) {
	tpl, err := ace.Load(common.GetConfig().Server.ViewPath+"/layout", common.GetConfig().Server.ViewPath+"/changelog", nil)
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
	isTXT := strings.HasSuffix(r.URL.Path, ".txt")
	data, err := readLogFile(common.GetConfig().LogPath + "/" + vars["channel"] + "/" + vars["month"] + "/" + vars["date"])
	if err != nil {
		serveError(w, err)
		return
	}

	w.Header().Set("Content-type", "text/plain; charset=UTF-8")
	if _, ok := vars["filter"]; ok {
		reader := bufio.NewReaderSize(bytes.NewReader(data), len(data))
		filteredData := bytes.NewBuffer([]byte{})
		var lineCount int
		for {
			line, err := reader.ReadSlice('\n')
			if err != nil {
				if err != io.EOF {
					log.Printf("error reading bytes %s", err)
				}
				break
			}
			if filterKey(line, vars["filter"]) {
				if !isTXT {
					filteredData.Write(line)
				} else {
					w.Write(line)
				}
				lineCount++
			}
		}
		if lineCount == 0 {
			serveError(w, ErrSearchKeyNotFound)
			return
		}
		if !isTXT {
			serveHTML(w, filteredData.Bytes())
			return
		}
		return
	}
	if !isTXT {
		serveHTML(w, data)
		return
	}
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
	isTXT := strings.HasSuffix(r.URL.Path, ".txt")
	if _, ok := vars["filter"]; ok {
		serveFilteredLogs(w, common.GetConfig().LogPath+"/"+vars["channel"]+"/"+vars["month"], searchKey(vars["nick"], vars["filter"]), isTXT)
		return
	}
	serveFilteredLogs(w, common.GetConfig().LogPath+"/"+vars["channel"]+"/"+vars["month"], nickFilter(vars["nick"]), isTXT)
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
	isTXT := strings.HasSuffix(r.URL.Path, ".txt")
	nick := bytes.ToLower([]byte(vars["nick"]))
	filter := func(line []byte) bool {
		return bytes.Contains(bytes.ToLower(line), nick)
	}
	serveFilteredLogs(w, common.GetConfig().LogPath+"/"+vars["channel"]+"/"+vars["month"], filter, isTXT)
}

// BroadcasterHandle channel index
func BroadcasterHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	isTXT := strings.HasSuffix(r.URL.Path, ".txt")
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
	serveFilteredLogs(w, common.GetConfig().LogPath+"/"+vars["channel"]+"/"+vars["month"], nickFilter(rs.Nick()), isTXT)
}

// SubscriberHandle channel index
func SubscriberHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	isTXT := strings.HasSuffix(r.URL.Path, ".txt")
	serveFilteredLogs(w, common.GetConfig().LogPath+"/"+vars["channel"]+"/"+vars["month"], nickFilter("twitchnotify"), isTXT)
}

// DestinyBroadcasterHandle destiny logs
func DestinyBroadcasterHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	isTXT := strings.HasSuffix(r.URL.Path, ".txt")
	serveFilteredLogs(w, common.GetConfig().LogPath+"/Destinygg chatlog/"+vars["month"], nickFilter("Destiny"), isTXT)
}

// DestinySubscriberHandle destiny subscriber logs
func DestinySubscriberHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	isTXT := strings.HasSuffix(r.URL.Path, ".txt")
	serveFilteredLogs(w, common.GetConfig().LogPath+"/Destinygg chatlog/"+vars["month"], nickFilter("Subscriber"), isTXT)
}

// DestinyBanHandle channel ban list
func DestinyBanHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	isTXT := strings.HasSuffix(r.URL.Path, ".txt")
	serveFilteredLogs(w, common.GetConfig().LogPath+"/Destinygg chatlog/"+vars["month"], nickFilter("Ban"), isTXT)
}

// DestinyBaseHandle shows the most recent months logs directly on the subdomain
func DestinyBaseHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	vars["channel"] = "Destinygg chatlog"
	vars["month"] = time.Now().Format("January 2006")
	MonthHandle(w, r)
}

// DestinyNickHandle shows the users most recent available log
func DestinyNickHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	vars["channel"] = "Destinygg chatlog"
	search, err := common.NewNickSearch(common.GetConfig().LogPath+"/"+vars["channel"], vars["nick"])
	if err != nil {
		serveError(w, ErrNotFound)
		return
	}
	rs, err := search.Next()
	if err != nil {
		serveError(w, ErrNotFound)
		return
	}
	if rs.Nick() != vars["nick"] {
		isTXT := strings.HasSuffix(r.URL.Path, ".txt")
		if isTXT {
			http.Redirect(w, r, "./"+rs.Nick()+".txt", 301)
		}
		http.Redirect(w, r, "./"+rs.Nick(), 301)
		return
	}
	vars["month"] = rs.Month()
	UserHandle(w, r)
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

func searchKey(nick, filter string) func([]byte) bool {
	nick += ":"
	return func(line []byte) bool {
		for i := 0; i < len(nick); i++ {
			if i+LogLinePrefixLength > len(line) || line[i+LogLinePrefixLength] != nick[i] {
				return false
			}
		}
		if bytes.Contains(bytes.ToLower(line[len(nick)+LogLinePrefixLength:]), bytes.ToLower([]byte(filter))) {
			return true
		}
		return false
	}
}

func filterKey(line []byte, f string) bool {
	if bytes.Contains(bytes.ToLower(line), bytes.ToLower([]byte(f))) {
		return true
	}
	return false
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
		icon := "file-text"
		if filepath.Ext(path) == "" {
			icon = "folder"
		}
		data["Paths"] = append(data["Paths"].([]map[string]string), map[string]string{
			"Path": basePath + strings.Replace(path, ".txt", "", -1),
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

func serveFilteredLogs(w http.ResponseWriter, path string, filter func([]byte) bool, isTXT bool) {
	logs, err := readLogDir(path)
	if err != nil {
		serveError(w, err)
		return
	}

	w.Header().Set("Content-type", "text/plain; charset=UTF-8")
	filteredData := bytes.NewBuffer([]byte{})
	var lineCount int
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
				if !isTXT {
					filteredData.Write(line)
				} else {
					w.Write(line)
				}
				lineCount++
			}
		}
	}
	if lineCount == 0 {
		serveError(w, ErrSearchKeyNotFound)
		return
	}
	if !isTXT {
		serveHTML(w, filteredData.Bytes())
	}
}

func serveHTML(w http.ResponseWriter, data []byte) {
	tpl, err := ace.Load(common.GetConfig().Server.ViewPath+"/layout", common.GetConfig().Server.ViewPath+"/content", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-type", "text/html; charset=UTF-8")
	if err := tpl.Execute(w, string(data)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
