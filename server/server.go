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
	"sort"
	"strconv"
	"strings"
	"sync"
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
	LogLinePrefixLength = len("[2017-01-10 08:57:47 UTC] ")
)

// errors
var (
	ErrUserNotFound      = errors.New("didn't find any logs for this user")
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
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	d := NewDebugger()
	r := mux.NewRouter()
	r.StrictSlash(true)
	r.HandleFunc("/", d.WatchHandle("Base", BaseHandle)).Methods("GET")
	r.HandleFunc("/contact", d.WatchHandle("Contact", ContactHandle)).Methods("GET")
	r.HandleFunc("/changelog", d.WatchHandle("Changelog", ChangelogHandle)).Methods("GET")
	r.HandleFunc("/mentions/{nick:[a-zA-Z0-9_-]{1,25}}.txt", d.WatchHandle("MentionsHandle", MentionsHandle)).Methods("GET").Queries("date", "{date:[0-9]{4}-[0-9]{2}-[0-9]{2}}")
	r.HandleFunc("/mentions/{nick:[a-zA-Z0-9_-]{1,25}}.txt", d.WatchHandle("MentionsHandle", MentionsHandle)).Methods("GET")
	r.HandleFunc("/mentions/{nick:[a-zA-Z0-9_-]{1,25}}", d.WatchHandle("MentionsHandle", WrapperHandle)).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+}/mentions/{nick:[a-zA-Z0-9_-]{1,25}}.txt", d.WatchHandle("MentionsHandle", MentionsHandle)).Methods("GET").Queries("date", "{date:[0-9]{4}-[0-9]{2}-[0-9]{2}}")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+}/mentions/{nick:[a-zA-Z0-9_-]{1,25}}.txt", d.WatchHandle("MentionsHandle", MentionsHandle)).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+}/mentions/{nick:[a-zA-Z0-9_-]{1,25}}", d.WatchHandle("MentionsHandle", WrapperHandle)).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}", d.WatchHandle("Channel", ChannelHandle)).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}", d.WatchHandle("Month", MonthHandle)).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/{date:[0-9]{4}-[0-9]{2}-[0-9]{2}}.txt", d.WatchHandle("Day", DayHandle)).Queries("search", "{filter:.+}").Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/{date:[0-9]{4}-[0-9]{2}-[0-9]{2}}.txt", d.WatchHandle("Day", DayHandle)).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/{date:[0-9]{4}-[0-9]{2}-[0-9]{2}}", d.WatchHandle("Day", WrapperHandle)).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/userlogs", d.WatchHandle("Users", UsersHandle)).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/userlogs/{nick:[a-zA-Z0-9_-]{1,25}}.txt", d.WatchHandle("User", UserHandle)).Queries("search", "{filter:.+}").Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/userlogs/{nick:[a-zA-Z0-9_-]{1,25}}.txt", d.WatchHandle("User", UserHandle)).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/userlogs/{nick:[a-zA-Z0-9_-]{1,25}}", d.WatchHandle("User", WrapperHandle)).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/premium/{nick:[a-zA-Z0-9_-]{1,25}}", d.WatchHandle("Premium", PremiumHandle)).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/premium/{nick:[a-zA-Z0-9_-]{1,25}}/{month:[a-zA-Z]+ [0-9]{4}}.txt", d.WatchHandle("PremiumUser", PremiumUserHandle)).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/premium/{nick:[a-zA-Z0-9_-]{1,25}}/{month:[a-zA-Z]+ [0-9]{4}}", d.WatchHandle("PremiumUser", WrapperHandle)).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/current", d.WatchHandle("CurrentBase", CurrentBaseHandle)).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/current/{nick:[a-zA-Z0-9_]+}.txt", d.WatchHandle("NickHandle", NickHandle)).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/current/{nick:[a-zA-Z0-9_]+}", d.WatchHandle("NickHandle", WrapperHandle)).Methods("GET")
	r.HandleFunc("/Destinygg chatlog/{month:[a-zA-Z]+ [0-9]{4}}/broadcaster.txt", d.WatchHandle("DestinyBroadcaster", DestinyBroadcasterHandle)).Methods("GET")
	r.HandleFunc("/Destinygg chatlog/{month:[a-zA-Z]+ [0-9]{4}}/broadcaster", d.WatchHandle("DestinyBroadcaster", WrapperHandle)).Methods("GET")
	r.HandleFunc("/Destinygg chatlog/{month:[a-zA-Z]+ [0-9]{4}}/subscribers.txt", d.WatchHandle("DestinySubscriber", DestinySubscriberHandle)).Methods("GET")
	r.HandleFunc("/Destinygg chatlog/{month:[a-zA-Z]+ [0-9]{4}}/subscribers", d.WatchHandle("DestinySubscriber", WrapperHandle)).Methods("GET")
	r.HandleFunc("/Destinygg chatlog/{month:[a-zA-Z]+ [0-9]{4}}/bans.txt", d.WatchHandle("DestinyBan", DestinyBanHandle)).Methods("GET")
	r.HandleFunc("/Destinygg chatlog/{month:[a-zA-Z]+ [0-9]{4}}/bans", d.WatchHandle("DestinyBan", WrapperHandle)).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/broadcaster.txt", d.WatchHandle("Broadcaster", BroadcasterHandle)).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/broadcaster", d.WatchHandle("Broadcaster", WrapperHandle)).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/subscribers.txt", d.WatchHandle("Subscriber", SubscriberHandle)).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/subscribers", d.WatchHandle("Subscriber", WrapperHandle)).Methods("GET")
	r.HandleFunc("/api/v1/channels.json", d.WatchHandle("Channels", ChannelsHandle)).Methods("GET")
	r.HandleFunc("/api/v1/stalk/{channel:[a-zA-Z0-9_-]+ chatlog}/{nick:[a-zA-Z0-9_-]+}.json", d.WatchHandle("Stalk", StalkHandle)).Queries("limit", "{limit:[0-9]+}").Methods("GET")
	r.HandleFunc("/api/v1/stalk/{channel:[a-zA-Z0-9_-]+ chatlog}/{nick:[a-zA-Z0-9_-]+}.json", d.WatchHandle("Stalk", StalkHandle)).Methods("GET")
	r.HandleFunc("/api/v1/status.json", d.WatchHandle("Debug", d.HTTPHandle))
	r.NotFoundHandler = http.HandlerFunc(NotFoundHandle)
	// r.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))

	srv := &http.Server{
		Addr:         common.GetConfig().Server.Address,
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	go srv.ListenAndServe()

	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
	<-sigint
	log.Println("i love you guys, be careful")
	os.Exit(0)
}

// Debugger logging...
type Debugger struct {
	mu       sync.Mutex
	counters map[string]*int64
}

// NewDebugger ...
func NewDebugger() *Debugger {
	d := &Debugger{counters: make(map[string]*int64)}
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
	var c *int64
	var ok bool
	d.mu.Lock()
	if c, ok = d.counters[name]; !ok {
		c = new(int64)
		d.counters[name] = c
	}
	d.mu.Unlock()
	return func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(c, 1)
		start := time.Now()
		f.ServeHTTP(w, r)
		log.Printf("served \"%s\" in %s", r.URL.Path, time.Since(start))
		atomic.AddInt64(c, -1)
	}
}

func (d *Debugger) counts() map[string]int64 {
	counts := make(map[string]int64)
	d.mu.Lock()
	for name, c := range d.counters {
		counts[name] = atomic.LoadInt64(c)
	}
	d.mu.Unlock()
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

// WrapperHandle static html log wrapper
func WrapperHandle(w http.ResponseWriter, r *http.Request) {
	tpl, err := ace.Load(common.GetConfig().Server.ViewPath+"/layout", common.GetConfig().Server.ViewPath+"/wrapper", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-type", "text/html; charset=UTF-8")
	path := r.URL.Path + ".txt"
	if r.URL.RawQuery != "" {
		path += "?" + r.URL.RawQuery
	}
	if err := tpl.Execute(w, struct{ Path string }{Path: path}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
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
	paths, err := readDirIndex(filepath.Join(common.GetConfig().LogPath, vars["channel"]))
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
	paths, err := readLogDir(filepath.Join(common.GetConfig().LogPath, vars["channel"], vars["month"]))
	if err != nil {
		serveError(w, err)
		return
	}
	metaPaths := []string{"userlogs", "broadcaster.txt", "subscribers.txt"}
	if vars["channel"] == "Destinygg chatlog" {
		metaPaths = append(metaPaths, "bans.txt")
	}
	sort.Sort(dirsByDay(paths))
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
	data, err := readLogFile(filepath.Join(common.GetConfig().LogPath, vars["channel"], vars["month"], vars["date"]))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-type", "text/plain; charset=UTF-8")
	w.Header().Set("Cache-control", "max-age=60")
	var ok bool
	var filter func([]byte, string) bool
	if _, ok = vars["filter"]; ok {
		filter = filterKey
	} else {
		filter = func(l []byte, f string) bool { return true }
	}
	var lineCount int
	reader := bufio.NewReaderSize(bytes.NewReader(data), len(data))
	for {
		line, err := reader.ReadSlice('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("error reading bytes %s", err)
			}
			break
		}
		if filter(line, vars["filter"]) {
			w.Write(line)
			lineCount++
		}
	}
	if lineCount == 0 && ok {
		http.Error(w, ErrSearchKeyNotFound.Error(), http.StatusNotFound)
	}
}

// UsersHandle channel index .
func UsersHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	f, err := os.Open(filepath.Join(common.GetConfig().LogPath, vars["channel"], vars["month"]))
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
			common.ReadNickList(nicks, filepath.Join(common.GetConfig().LogPath, vars["channel"], vars["month"], file.Name()))
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
	if _, ok := vars["filter"]; ok {
		serveFilteredLogs(w, filepath.Join(common.GetConfig().LogPath, vars["channel"], vars["month"]), searchKey(vars["nick"], vars["filter"]))
		return
	}
	serveFilteredLogs(w, filepath.Join(common.GetConfig().LogPath, vars["channel"], vars["month"]), nickFilter(vars["nick"]))
}

// PremiumHandle premium user log index
func PremiumHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	paths, err := readDirIndex(filepath.Join(common.GetConfig().LogPath, vars["channel"]))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
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
	serveFilteredLogs(w, filepath.Join(common.GetConfig().LogPath, vars["channel"], vars["month"]), filter)
}

// BroadcasterHandle channel index
func BroadcasterHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	nick := vars["channel"][:len(vars["channel"])-8]
	search, err := common.NewNickSearch(filepath.Join(common.GetConfig().LogPath, vars["channel"]), nick)
	if err != nil {
		http.Error(w, ErrUserNotFound.Error(), http.StatusNotFound)
		return
	}
	rs, err := search.Next()
	if err == io.EOF {
		http.Error(w, ErrUserNotFound.Error(), http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	serveFilteredLogs(w, filepath.Join(common.GetConfig().LogPath, vars["channel"], vars["month"]), nickFilter(rs.Nick()))
}

// SubscriberHandle channel index
func SubscriberHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serveFilteredLogs(w, filepath.Join(common.GetConfig().LogPath, vars["channel"], vars["month"]), nickFilter("twitchnotify"))
}

// DestinyBroadcasterHandle destiny logs
func DestinyBroadcasterHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serveFilteredLogs(w, filepath.Join(common.GetConfig().LogPath, "Destinygg chatlog", vars["month"]), nickFilter("Destiny"))
}

// DestinySubscriberHandle destiny subscriber logs
func DestinySubscriberHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serveFilteredLogs(w, filepath.Join(common.GetConfig().LogPath, "Destinygg chatlog", vars["month"]), nickFilter("Subscriber"))
}

// DestinyBanHandle channel ban list
func DestinyBanHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serveFilteredLogs(w, filepath.Join(common.GetConfig().LogPath, "Destinygg chatlog", vars["month"]), nickFilter("Ban"))
}

// CurrentBaseHandle shows the most recent months logs directly on the subdomain
func CurrentBaseHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	vars["month"] = time.Now().Format("January 2006")
	MonthHandle(w, r)
}

func convertChannelCase(ch string) string {
	ch = strings.ToLower(ch)
	p := strings.Split(ch, " ")
	p[0] = strings.Title(p[0])
	return strings.Join(p, " ")
}

// NickHandle shows the users most recent available log
func NickHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	vars["channel"] = convertChannelCase(vars["channel"])
	search, err := common.NewNickSearch(filepath.Join(common.GetConfig().LogPath, vars["channel"]), vars["nick"])
	if err != nil {
		http.Error(w, ErrUserNotFound.Error(), http.StatusNotFound)
		return
	}
	rs, err := search.Next()
	if err != nil {
		http.Error(w, ErrUserNotFound.Error(), http.StatusNotFound)
		return
	}
	if rs.Nick() != vars["nick"] {
		http.Redirect(w, r, "./"+rs.Nick()+".txt", 301)
		return
	}
	vars["month"] = rs.Month()
	UserHandle(w, r)
}

// MentionsHandle shows each line where a specific nick gets mentioned
func MentionsHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	if _, ok := vars["channel"]; ok {
		vars["channel"] = strings.Title(vars["channel"]) + " chatlog"
	} else {
		vars["channel"] = "Destinygg chatlog"
	}
	if _, ok := vars["date"]; !ok {
		vars["date"] = time.Now().UTC().Format("2006-01-02")
	}
	t, err := time.Parse("2006-01-02", vars["date"])
	if err != nil {
		http.Error(w, "invalid date format", http.StatusNotFound)
		return
	}
	if t.After(time.Now().UTC()) {
		http.Error(w, "can't look into the future D:", http.StatusNotFound)
		return
	}
	data, err := readLogFile(filepath.Join(common.GetConfig().LogPath, vars["channel"], t.Format("January 2006"), t.Format("2006-01-02")))
	if err != nil {
		http.Error(w, "no logs found :(", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-type", "text/plain; charset=UTF-8")
	var lineCount int
	reader := bufio.NewReaderSize(bytes.NewReader(data), len(data))
	for {
		line, err := reader.ReadSlice('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("error reading bytes %s", err)
			}
			break
		}
		lowerLine := bytes.ToLower(line)
		if bytes.Contains(lowerLine[bytes.Index(lowerLine[LogLinePrefixLength:], []byte(":"))+LogLinePrefixLength:], bytes.ToLower([]byte(" "+vars["nick"]))) {
			w.Write(line)
			lineCount++
		}
	}
	if lineCount == 0 {
		http.Error(w, "no mentions :(", http.StatusNotFound)
	}
}

func ChannelsHandle(w http.ResponseWriter, r *http.Request) {
	type Error struct {
		Error string `json:"error"`
	}

	w.Header().Set("Content-type", "application/json")
	dirs, err := filepath.Glob(filepath.Join(common.GetConfig().LogPath, "*"))
	if err != nil {
		d, _ := json.Marshal(Error{err.Error()})
		http.Error(w, string(d), http.StatusInternalServerError)
		return
	}

	for i, v := range dirs {
		dirs[i] = v[len(common.GetConfig().LogPath)+1 : len(v)-8]
	}
	d, _ := json.MarshalIndent(dirs, "", "\t")
	w.Write(d)
}

// StalkHandle return n most recent lines of chat for user
func StalkHandle(w http.ResponseWriter, r *http.Request) {
	type Error struct {
		Error string `json:"error"`
	}

	w.Header().Set("Content-type", "application/json")
	vars := mux.Vars(r)
	if _, ok := vars["limit"]; !ok {
		vars["limit"] = "3"
	}
	limit, err := strconv.ParseUint(vars["limit"], 10, 32)
	if err != nil {
		d, _ := json.Marshal(Error{err.Error()})
		http.Error(w, string(d), http.StatusBadRequest)
		return
	}
	if limit > uint64(common.GetConfig().Server.MaxStalkLines) {
		limit = uint64(common.GetConfig().Server.MaxStalkLines)
	} else if limit < 1 {
		limit = 3
	}
	buf := make([]string, limit)
	index := limit
	search, err := common.NewNickSearch(filepath.Join(common.GetConfig().LogPath, vars["channel"]), vars["nick"])
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
		data, err := readLogFile(filepath.Join(common.GetConfig().LogPath, vars["channel"], rs.Month(), rs.Day()))
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
	return !b.After(a)
}

type dirsByDay []string

func (l dirsByDay) Len() int {
	return len(l)
}

func (l dirsByDay) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func (l dirsByDay) Less(i, j int) bool {
	format := "2006-01-02.txt.lz4"
	a, err := time.Parse(format, lz4Path(l[i]))
	if err != nil {
		log.Println(l[i])
		return true
	}
	b, err := time.Parse(format, lz4Path(l[j]))
	if err != nil {
		log.Println(l[j])
		return false
	}
	return !b.After(a)
}

func lz4Path(path string) string {
	if path[len(path)-4:] != ".lz4" {
		path += ".lz4"
	}
	return path
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
		return bytes.Contains(bytes.ToLower(line[len(nick)+LogLinePrefixLength:]), bytes.ToLower([]byte(filter)))
	}
}

func filterKey(line []byte, f string) bool {
	return bytes.Contains(bytes.ToLower(line), bytes.ToLower([]byte(f)))
}

// serveError ...
func serveError(w http.ResponseWriter, e error) {
	tpl, err := ace.Load(filepath.Join(common.GetConfig().Server.ViewPath, "layout"), filepath.Join(common.GetConfig().Server.ViewPath, "error"), nil)
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
	tpl, err := ace.Load(filepath.Join(common.GetConfig().Server.ViewPath, "layout"), filepath.Join(common.GetConfig().Server.ViewPath, "directory"), nil)
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

func serveFilteredLogs(w http.ResponseWriter, path string, filter func([]byte) bool) {
	logs, err := readLogDir(path)
	if err != nil {
		http.Error(w, ErrNotFound.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-type", "text/plain; charset=UTF-8")
	var lineCount int
	for _, name := range logs {
		data, err := readLogFile(filepath.Join(path, name))
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
				lineCount++
			}
		}
	}
	if lineCount == 0 {
		http.Error(w, ErrSearchKeyNotFound.Error(), http.StatusNotFound)
		return
	}
}
