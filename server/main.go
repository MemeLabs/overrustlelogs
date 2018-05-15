package main

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
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
	"syscall"
	"time"

	"github.com/CloudyKit/jet"
	"github.com/MemeLabs/overrustlelogs/common"
	"github.com/fatih/color"
	"github.com/gorilla/mux"
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
	LogExtension   = regexp.MustCompile(`\.txt(\.gz)?$`)
	NicksExtension = regexp.MustCompile(`\.nicks\.gz$`)

	green = color.New(color.FgGreen).SprintFunc()
	blue  = color.New(color.FgBlue).SprintFunc()
	cyan  = color.New(color.FgCyan).SprintFunc()
)

var view *jet.Set

func init() {
	configPath := flag.String("config", "", "config path")
	flag.Parse()
	common.SetupConfig(*configPath)
}

// Start server
func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	view = jet.NewHTMLSet(common.GetConfig().Server.ViewPath)
	view.SetDevelopmentMode(true)

	r := mux.NewRouter()
	r.StrictSlash(true)
	r.HandleFunc("/", BaseHandle).Methods("GET")
	r.HandleFunc("/contact", ContactHandle).Methods("GET")
	r.HandleFunc("/changelog", ChangelogHandle).Methods("GET")
	r.HandleFunc("/stalk", StalkerHandle).Methods("GET").Queries("channel", "{channel:[a-zA-Z0-9_-]+}", "nick", "{nick:@?[a-zA-Z0-9_-]+}")
	r.HandleFunc("/stalk", StalkerHandle).Methods("GET")
	r.HandleFunc("/mentions/{nick:[a-zA-Z0-9_-]{1,25}}.txt", MentionsHandle).Methods("GET").Queries("date", "{date:[0-9]{4}-[0-9]{2}-[0-9]{2}}")
	r.HandleFunc("/mentions/{nick:[a-zA-Z0-9_-]{1,25}}.txt", MentionsHandle).Methods("GET")
	r.HandleFunc("/mentions/{nick:[a-zA-Z0-9_-]{1,25}}", WrapperHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+}/mentions/{nick:[a-zA-Z0-9_-]{1,25}}.txt", MentionsHandle).Methods("GET").Queries("date", "{date:[0-9]{4}-[0-9]{2}-[0-9]{2}}")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+}/mentions/{nick:[a-zA-Z0-9_-]{1,25}}.txt", MentionsHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+}/mentions/{nick:[a-zA-Z0-9_-]{1,25}}", WrapperHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}", ChannelHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}", MonthHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/{date:[0-9]{4}-[0-9]{2}-[0-9]{2}}.txt", DayHandle).Queries("search", "{filter:.+}").Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/{date:[0-9]{4}-[0-9]{2}-[0-9]{2}}.txt", DayHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/{date:[0-9]{4}-[0-9]{2}-[0-9]{2}}", WrapperHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/top{limit:[0-9]{1,9}}", TopListHandle).Methods("GET").Queries("sort", "{sort:[a-z]+}")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/top{limit:[0-9]{1,9}}", TopListHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/userlogs", UsersHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/userlogs/{nick:[a-zA-Z0-9_-]{1,25}}.txt", UserHandle).Queries("search", "{filter:.+}").Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/userlogs/{nick:[a-zA-Z0-9_-]{1,25}}.txt", UserHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/userlogs/{nick:[a-zA-Z0-9_-]{1,25}}", WrapperHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/current", CurrentBaseHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/current/{nick:[a-zA-Z0-9_]+}.txt", NickHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/current/{nick:[a-zA-Z0-9_]+}", WrapperHandle).Methods("GET")
	r.HandleFunc("/Destinygg chatlog/{month:[a-zA-Z]+ [0-9]{4}}/broadcaster.txt", DestinyBroadcasterHandle).Methods("GET")
	r.HandleFunc("/Destinygg chatlog/{month:[a-zA-Z]+ [0-9]{4}}/broadcaster", WrapperHandle).Methods("GET")
	r.HandleFunc("/Destinygg chatlog/{month:[a-zA-Z]+ [0-9]{4}}/subscribers.txt", DestinySubscriberHandle).Methods("GET")
	r.HandleFunc("/Destinygg chatlog/{month:[a-zA-Z]+ [0-9]{4}}/subscribers", WrapperHandle).Methods("GET")
	r.HandleFunc("/Destinygg chatlog/{month:[a-zA-Z]+ [0-9]{4}}/bans.txt", DestinyBanHandle).Methods("GET")
	r.HandleFunc("/Destinygg chatlog/{month:[a-zA-Z]+ [0-9]{4}}/bans", WrapperHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/broadcaster.txt", BroadcasterHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/broadcaster", WrapperHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/subscribers.txt", SubscriberHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/subscribers", WrapperHandle).Methods("GET")
	r.NotFoundHandler = http.HandlerFunc(NotFoundHandle)
	// r.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", http.FileServer(http.Dir("./assets"))))

	api := r.PathPrefix("/api/v1").Subrouter()
	api.HandleFunc("/channels.json", ChannelsAPIHandle).Methods("GET")
	api.HandleFunc("/{channel:[a-zA-Z0-9_-]+}/months.json", MonthsAPIHandle).Methods("GET")
	api.HandleFunc("/{channel:[a-zA-Z0-9_-]+}/{month:[a-zA-Z]+ [0-9]{4}}/days.json", DaysAPIHandle).Methods("GET")
	api.HandleFunc("/{channel:[a-zA-Z0-9_-]+}/{month:[a-zA-Z]+ [0-9]{4}}/users.json", UsersAPIHandle).Methods("GET")
	api.HandleFunc("/{channel:[a-zA-Z0-9_-]+} chatlog/{month:[a-zA-Z]+ [0-9]{4}}/lines.json", LinesAPIHandle).Methods("GET")
	api.HandleFunc("/stalk/{channel:[a-zA-Z0-9_-]+}/{nick:[a-zA-Z0-9_-]+}.json", StalkHandle).Queries("limit", "{limit:[0-9]+}").Methods("GET")
	api.HandleFunc("/stalk/{channel:[a-zA-Z0-9_-]+}/{nick:[a-zA-Z0-9_-]+}.json", StalkHandle).Methods("GET")
	api.HandleFunc("/mentions/{channel:[a-zA-Z0-9_-]+}/{nick:[a-zA-Z0-9_-]+}.json", MentionsAPIHandle).Queries("date", "{date:[0-9]{4}-[0-9]{2}-[0-9]{2}}", "limit", "{limit:[0-9]+}").Methods("GET")
	api.HandleFunc("/mentions/{channel:[a-zA-Z0-9_-]+}/{nick:[a-zA-Z0-9_-]+}.json", MentionsAPIHandle).Queries("limit", "{limit:[0-9]+}").Methods("GET")
	api.HandleFunc("/mentions/{channel:[a-zA-Z0-9_-]+}/{nick:[a-zA-Z0-9_-]+}.json", MentionsAPIHandle).Queries("date", "{date:[0-9]{4}-[0-9]{2}-[0-9]{2}}").Methods("GET")
	api.HandleFunc("/mentions/{channel:[a-zA-Z0-9_-]+}/{nick:[a-zA-Z0-9_-]+}.json", MentionsAPIHandle).Methods("GET")
	api.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/top{limit:[0-9]{1,9}}.json", TopListApiHandle).Methods("GET").Queries("sort", "{sort:[a-z]+}")
	api.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/top{limit:[0-9]{1,9}}.json", TopListApiHandle).Methods("GET")

	srv := &http.Server{
		Addr:         common.GetConfig().Server.Address,
		Handler:      logger(r),
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

func logger(h http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		h.ServeHTTP(w, r)
		if strings.HasPrefix(r.URL.Path, "/assets/") || strings.HasPrefix(r.URL.Path, "/css/") || strings.HasPrefix(r.URL.Path, "/js/") {
			return
		}
		path := green(r.URL.Path)
		if r.URL.RawQuery != "" {
			path += blue("?" + r.URL.RawQuery)
		}
		fmt.Printf("served \"%s\" to \"%s\" in %s\n", path, r.Header.Get("Cf-Connecting-Ip"), cyan(time.Since(start)))
	}
}

// NotFoundHandle channel index
func NotFoundHandle(w http.ResponseWriter, r *http.Request) {
	serveError(w, r, ErrNotFound)
}

// BaseHandle channel index
func BaseHandle(w http.ResponseWriter, r *http.Request) {
	paths, err := readDirIndex(common.GetConfig().LogPath)
	if err != nil {
		serveError(w, r, err)
		return
	}
	serveDirIndex(w, []string{}, paths)
}

// WrapperHandle static html log wrapper
func WrapperHandle(w http.ResponseWriter, r *http.Request) {
	tpl, err := view.GetTemplate("wrapper")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-type", "text/html; charset=UTF-8")
	path := r.URL.Path + ".txt"
	if r.URL.RawQuery != "" {
		path += "?" + r.URL.RawQuery
	}
	if err := tpl.Execute(w, nil, struct{ Path string }{Path: path}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// ContactHandle contact page
func ContactHandle(w http.ResponseWriter, r *http.Request) {
	tpl, err := view.GetTemplate("contact")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-type", "text/html")
	if err := tpl.Execute(w, nil, nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// ChangelogHandle changelog page
func ChangelogHandle(w http.ResponseWriter, r *http.Request) {
	tpl, err := view.GetTemplate("changelog")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-type", "text/html")
	if err := tpl.Execute(w, nil, nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// ChannelHandle channel index
func ChannelHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	paths, err := readDirIndex(filepath.Join(common.GetConfig().LogPath, vars["channel"]))
	if err != nil {
		serveError(w, r, err)
		return
	}
	sort.Sort(byMonth(paths))
	serveDirIndex(w, []string{vars["channel"]}, paths)
}

// MonthHandle channel index
func MonthHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	paths, err := readLogDir(filepath.Join(common.GetConfig().LogPath, convertChannelCase(vars["channel"]), vars["month"]))
	if err != nil {
		serveError(w, r, err)
		return
	}
	metaPaths := []string{"userlogs", "broadcaster.txt", "subscribers.txt"}
	if vars["channel"] == "Destinygg chatlog" {
		metaPaths = append(metaPaths, "bans.txt")
	}
	sort.Sort(byDay(paths))
	paths = append(paths, metaPaths...)
	copy(paths[len(metaPaths):], paths)
	copy(paths, metaPaths)
	for i, path := range paths {
		paths[i] = LogExtension.ReplaceAllString(path, ".txt")
	}
	serveDirIndex(w, []string{convertChannelCase(vars["channel"]), vars["month"]}, paths)
}

// DayHandle channel index
func DayHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	data, err := readLogFile(filepath.Join(common.GetConfig().LogPath, convertChannelCase(vars["channel"]), vars["month"], vars["date"]))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-type", "text/plain; charset=UTF-8")
	w.Header().Set("Cache-control", "max-age=60")
	var ok bool
	var filter = func(l []byte, f string) bool { return true }
	if _, ok = vars["filter"]; ok {
		filter = filterKey
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
	f, err := os.Open(filepath.Join(common.GetConfig().LogPath, convertChannelCase(vars["channel"]), vars["month"]))
	if err != nil {
		serveError(w, r, ErrNotFound)
		return
	}
	files, err := f.Readdir(0)
	if err != nil {
		serveError(w, r, err)
		return
	}
	nicks := common.NickList{}
	for _, file := range files {
		if NicksExtension.MatchString(file.Name()) {
			common.ReadNickList(nicks, filepath.Join(common.GetConfig().LogPath, convertChannelCase(vars["channel"]), vars["month"], file.Name()))
		}
	}
	names := make([]string, 0, len(nicks))
	for nick := range nicks {
		names = append(names, nick+".txt")
	}
	sort.Strings(names)
	serveDirIndex(w, []string{convertChannelCase(vars["channel"]), vars["month"], "userlogs"}, names)
}

// UserHandle user log
func UserHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	vars["channel"] = convertChannelCase(vars["channel"])
	nick, ok := userInMonth(vars["channel"], vars["nick"], vars["month"])
	if !ok {
		http.Error(w, ErrUserNotFound.Error(), http.StatusNotFound)
		return
	}
	if _, ok := vars["filter"]; ok {
		serveFilteredLogs(w, filepath.Join(common.GetConfig().LogPath, vars["channel"], vars["month"]), searchKey(nick, vars["filter"]))
		return
	}
	serveFilteredLogs(w, filepath.Join(common.GetConfig().LogPath, vars["channel"], vars["month"]), nickFilter(nick))
}

func userInMonth(channel, nick, month string) (string, bool) {
	search, err := common.NewNickSearch(filepath.Join(common.GetConfig().LogPath, channel), nick)
	if err != nil {
		return "", false
	}
	n, err := search.Month(month)
	if err != nil {
		return "", false
	}
	return n, true
}

// BroadcasterHandle channel index
func BroadcasterHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	vars["channel"] = convertChannelCase(vars["channel"])
	nick := vars["channel"][:len(vars["channel"])-8]
	nick, ok := userInMonth(vars["channel"], nick, vars["month"])
	if !ok {
		http.Error(w, ErrUserNotFound.Error(), http.StatusInternalServerError)
		return
	}
	serveFilteredLogs(w, filepath.Join(common.GetConfig().LogPath, vars["channel"], vars["month"]), nickFilter(nick))
}

// SubscriberHandle channel index
func SubscriberHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	vars["channel"] = convertChannelCase(vars["channel"])
	nick, ok := userInMonth(vars["channel"], "twitchnotify", vars["month"])
	if !ok {
		http.Error(w, errors.New("no subscribers this month :(").Error(), http.StatusInternalServerError)
		return
	}
	serveFilteredLogs(w, filepath.Join(common.GetConfig().LogPath, vars["channel"], vars["month"]), nickFilter(nick))
}

// DestinyBroadcasterHandle destiny logs
func DestinyBroadcasterHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	vars["channel"] = "Destinygg chatlog"
	nick, ok := userInMonth(vars["channel"], "Destiny", vars["month"])
	if !ok {
		http.Error(w, ErrUserNotFound.Error(), http.StatusInternalServerError)
		return
	}
	serveFilteredLogs(w, filepath.Join(common.GetConfig().LogPath, vars["channel"], vars["month"]), nickFilter(nick))
}

// DestinySubscriberHandle destiny subscriber logs
func DestinySubscriberHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	vars["channel"] = "Destinygg chatlog"
	nick, ok := userInMonth(vars["channel"], "Subscriber", vars["month"])
	if !ok {
		http.Error(w, errors.New("no subscribers this month").Error(), http.StatusInternalServerError)
		return
	}
	serveFilteredLogs(w, filepath.Join(common.GetConfig().LogPath, vars["channel"], vars["month"]), nickFilter(nick))
}

// DestinyBanHandle channel ban list
func DestinyBanHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	vars["channel"] = "Destinygg chatlog"
	nick, ok := userInMonth(vars["channel"], "Ban", vars["month"])
	if !ok {
		http.Error(w, ErrUserNotFound.Error(), http.StatusInternalServerError)
		return
	}
	serveFilteredLogs(w, filepath.Join(common.GetConfig().LogPath, vars["channel"], vars["month"]), nickFilter(nick))
}

// CurrentBaseHandle shows the most recent months logs directly on the subdomain
func CurrentBaseHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	vars["month"] = time.Now().Format("January 2006")
	MonthHandle(w, r)
}

func convertChannelCase(ch string) string {
	if strings.Contains(ch, " chatlog") {
		ch = ch[:len(ch)-8]
	}
	return strings.Title(strings.ToLower(ch)) + " chatlog"
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
		vars["channel"] = strings.Title(strings.ToLower(vars["channel"])) + " chatlog"
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
	data, err := readLogFile(filepath.Join(common.GetConfig().LogPath, convertChannelCase(vars["channel"]), t.Format("January 2006"), t.Format("2006-01-02")))
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
		if isMentioned([]byte(" "+vars["nick"]), lowerLine) {
			w.Write(line)
			lineCount++
		}
	}
	if lineCount == 0 {
		http.Error(w, "no mentions :(", http.StatusNotFound)
	}
}

func isMentioned(nick, line []byte) bool {
	colonIndex := bytes.Index(line[LogLinePrefixLength:], []byte(":")) + LogLinePrefixLength
	return bytes.Contains(line[colonIndex:], bytes.ToLower([]byte(string(nick)+" "))) || bytes.Contains(line[colonIndex:], bytes.ToLower([]byte(string(nick)+"\n")))
}

// MentionsAPIHandle returns mentions from a nick in json format
func MentionsAPIHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	if _, ok := vars["channel"]; ok {
		vars["channel"] = strings.Title(strings.ToLower(vars["channel"])) + " chatlog"
	} else {
		vars["channel"] = "Destinygg chatlog"
	}

	w.Header().Set("Content-type", "text/plain")
	if _, ok := vars["date"]; !ok {
		vars["date"] = time.Now().UTC().Format("2006-01-02")
	}
	t, err := time.Parse("2006-01-02", vars["date"])
	if err != nil {
		http.Error(w, "invalid date format", http.StatusBadRequest)
		return
	}
	if t.After(time.Now().UTC()) {
		http.Error(w, "can't look into the future D:", http.StatusBadRequest)
		return
	}

	data, err := readLogFile(filepath.Join(common.GetConfig().LogPath, convertChannelCase(vars["channel"]), t.Format("January 2006"), t.Format("2006-01-02")))
	if err != nil {
		http.Error(w, "no logs found :( ", http.StatusNotFound)
		return
	}

	var lines [][]byte
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
		if isMentioned([]byte(" "+vars["nick"]), lowerLine) {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		http.Error(w, "no mentions :(", http.StatusNotFound)
		return
	}

	var limit int
	_, ok := vars["limit"]
	if !ok {
		limit = len(lines)
	} else {
		l, err := strconv.Atoi(vars["limit"])
		if err != nil {
			log.Println(err)
			http.Error(w, "limit query is not a integer", http.StatusBadRequest)
			return
		}
		limit = l
	}

	var buf = lines[len(lines)-limit:]

	type msg struct {
		Date int64  `json:"date"`
		Text string `json:"text"`
		Nick string `json:"nick"`
	}

	mentions := make([]msg, 0)
	for _, line := range buf {
		t, err := time.Parse("2006-01-02 15:04:05 MST", string(line[1:24]))
		if err != nil {
			continue
		}

		i := bytes.Index(line[LogLinePrefixLength:], []byte(":"))
		data := msg{
			Date: t.Unix(),
			Nick: string(line[LogLinePrefixLength : LogLinePrefixLength+i]),
			Text: strings.TrimSpace(string(line[i+LogLinePrefixLength+2:])),
		}
		mentions = append(mentions, data)
	}

	w.Header().Set("Content-type", "application/json")
	d, _ := json.Marshal(mentions)
	w.Write(d)
}

// ChannelsAPIHandle lists the channels
func ChannelsAPIHandle(w http.ResponseWriter, r *http.Request) {
	type Error struct {
		Error string `json:"error"`
	}

	w.Header().Set("Content-type", "application/json")
	f, err := os.Open(common.GetConfig().LogPath)
	if err != nil {
		d, _ := json.Marshal(Error{err.Error()})
		http.Error(w, string(d), http.StatusInternalServerError)
		return
	}
	files, err := f.Readdirnames(0)
	if err != nil {
		d, _ := json.Marshal(Error{err.Error()})
		http.Error(w, string(d), http.StatusInternalServerError)
		return
	}

	for i, v := range files {
		files[i] = v[:len(v)-8]
	}
	sort.Strings(files)
	d, _ := json.Marshal(files)
	w.Write(d)
}

// MonthsAPIHandle lists the channels
func MonthsAPIHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	type Error struct {
		Error string `json:"error"`
	}

	w.Header().Set("Content-type", "application/json")
	f, err := os.Open(filepath.Join(common.GetConfig().LogPath, strings.Title(strings.ToLower(vars["channel"]))+" chatlog"))
	if err != nil {
		d, _ := json.Marshal(Error{err.Error()})
		http.Error(w, string(d), http.StatusInternalServerError)
		return
	}
	files, err := f.Readdirnames(0)
	if err != nil {
		d, _ := json.Marshal(Error{err.Error()})
		http.Error(w, string(d), http.StatusInternalServerError)
		return
	}

	sort.Sort(byMonth(files))
	d, _ := json.Marshal(files)
	w.Write(d)
}

// DaysAPIHandle lists the channels
func DaysAPIHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	type Error struct {
		Error string `json:"error"`
	}

	w.Header().Set("Content-type", "application/json")
	f, err := os.Open(filepath.Join(common.GetConfig().LogPath, strings.Title(strings.ToLower(vars["channel"]))+" chatlog", vars["month"]))
	if err != nil {
		d, _ := json.Marshal(Error{err.Error()})
		http.Error(w, string(d), http.StatusInternalServerError)
		return
	}
	files, err := f.Readdirnames(0)
	if err != nil {
		d, _ := json.Marshal(Error{err.Error()})
		http.Error(w, string(d), http.StatusInternalServerError)
		return
	}

	metaLogs := []string{"broadcaster.txt", "subscribers.txt"}
	if strings.EqualFold(convertChannelCase(vars["channel"]), "destinygg") {
		metaLogs = append(metaLogs, "bans.txt")
	}

	var temp []string
	for _, v := range files {
		if strings.Contains(v, ".nicks") {
			continue
		}
		if strings.Contains(v, ".gz") {
			temp = append(temp, v[:len(v)-3])
		}
	}
	var filteredDirs []string
	filteredDirs = append(filteredDirs, metaLogs...)

	sort.Sort(byDay(temp))
	filteredDirs = append(filteredDirs, temp...)

	d, _ := json.Marshal(filteredDirs)
	w.Write(d)
}

// LinesAPIHandle lists the channels
func LinesAPIHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	type Error struct {
		Error string `json:"error"`
	}

	w.Header().Set("Content-type", "application/json")
	monthPath := filepath.Join(common.GetConfig().LogPath, strings.Title(strings.ToLower(vars["channel"]))+" chatlog", vars["month"])
	f, err := os.Open(monthPath)
	if err != nil {
		d, _ := json.Marshal(Error{err.Error()})
		http.Error(w, string(d), http.StatusInternalServerError)
		return
	}
	files, err := f.Readdirnames(0)
	if err != nil {
		d, _ := json.Marshal(Error{err.Error()})
		http.Error(w, string(d), http.StatusInternalServerError)
		return
	}
	var temp linesData
	for _, v := range files {
		if strings.Contains(v, ".nicks") {
			continue
		}
		if strings.Contains(v, ".txt.gz") {
			b, err := common.ReadCompressedFile(filepath.Join(monthPath, v))
			if err != nil {
				continue
			}
			lines := bytes.Count(b, []byte("\n"))
			df := "2006-01-02"
			d, err := time.Parse(df, v[:len(df)])
			if err != nil {
				continue
			}
			date := d.UTC().UTC().Format("2006-01-02T15:04:05-0700")
			temp.Data = append(temp.Data, struct {
				Date  string `json:"date"`
				Lines int    `json:"lines"`
			}{Date: date, Lines: lines})
		}
	}
	sort.Sort(ByDate(temp))

	d, _ := json.Marshal(temp)
	w.Write(d)
}

type ByDate linesData

type linesData struct {
	Data []struct {
		Date  string `json:"date"`
		Lines int    `json:"lines"`
	} `json:"data"`
}

func (a ByDate) Len() int      { return len(a.Data) }
func (a ByDate) Swap(i, j int) { a.Data[i], a.Data[j] = a.Data[j], a.Data[i] }
func (a ByDate) Less(i, j int) bool {
	ad, _ := time.Parse("2006-01-02T15:04:05-0700", a.Data[i].Date)
	bd, _ := time.Parse("2006-01-02T15:04:05-0700", a.Data[j].Date)
	return ad.Before(bd)
}

// UsersAPIHandle returns the */userlogs directory in json format
func UsersAPIHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	f, err := os.Open(filepath.Join(common.GetConfig().LogPath, strings.Title(strings.ToLower(vars["channel"]))+" chatlog", vars["month"]))
	if err != nil {
		serveError(w, r, ErrNotFound)
		return
	}
	files, err := f.Readdirnames(0)
	if err != nil {
		serveError(w, r, err)
		return
	}
	nicks := common.NickList{}
	for _, file := range files {
		if NicksExtension.MatchString(file) {
			common.ReadNickList(nicks, filepath.Join(common.GetConfig().LogPath, strings.Title(strings.ToLower(vars["channel"]))+" chatlog", vars["month"], file))
		}
	}
	names := make([]string, 0, len(nicks))
	for nick := range nicks {
		names = append(names, nick+".txt")
	}
	sort.Strings(names)
	d, _ := json.Marshal(names)
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
	vars["channel"] = strings.Title(strings.ToLower(vars["channel"])) + " chatlog"
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
		data, err := readLogFile(filepath.Join(common.GetConfig().LogPath, convertChannelCase(vars["channel"]), rs.Month(), rs.Day()))
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

type byMonth []string

func (l byMonth) Len() int      { return len(l) }
func (l byMonth) Swap(i, j int) { l[i], l[j] = l[j], l[i] }
func (l byMonth) Less(i, j int) bool {
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

type byDay []string

func (l byDay) Len() int      { return len(l) }
func (l byDay) Swap(i, j int) { l[i], l[j] = l[j], l[i] }
func (l byDay) Less(i, j int) bool {
	format := "2006-01-02.txt.gz"
	a, err := time.Parse(format, gzPath(l[i]))
	if err != nil {
		return true
	}
	b, err := time.Parse(format, gzPath(l[j]))
	if err != nil {
		return false
	}
	return !b.After(a)
}

func gzPath(path string) string {
	if path[len(path)-3:] != ".gz" {
		path += ".gz"
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
	sort.Strings(names)
	return names, nil
}

func readLogDir(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, ErrNotFound
	}
	files, err := f.Readdirnames(0)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, file := range files {
		if LogExtension.MatchString(file) {
			names = append(names, file)
		}
	}
	sort.Strings(names)
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
	nick = strings.ToLower(nick)
	return func(line []byte) bool {
		if LogLinePrefixLength+len(nick) > len(line) {
			return false
		}
		if !bytes.EqualFold(line[LogLinePrefixLength:LogLinePrefixLength+len(nick)], []byte(nick)) {
			return false
		}
		return true
	}
}

func searchKey(nick, filter string) func([]byte) bool {
	nick += ":"
	nick = strings.ToLower(nick)
	filter = strings.ToLower(filter)
	return func(line []byte) bool {
		line = bytes.ToLower(line)
		if len(line) < LogLinePrefixLength+len(nick) {
			return false
		}
		if !bytes.HasPrefix(line[LogLinePrefixLength:], []byte(nick)) {
			return false
		}
		return bytes.Contains(line[LogLinePrefixLength+len(nick):], []byte(filter))
	}
}

func filterKey(line []byte, f string) bool {
	return bytes.Contains(bytes.ToLower(line), bytes.ToLower([]byte(f)))
}

// serveError ...
func serveError(w http.ResponseWriter, r *http.Request, e error) {
	tpl, err := view.GetTemplate("error")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-type", "text/html")
	if e == ErrNotFound {
		w.WriteHeader(http.StatusNotFound)
	} else if e != nil {
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
		e = errors.New("Unknown Error")
	}
	if err := tpl.Execute(w, nil, e.Error()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type (
	directoryPayload struct {
		Breadcrumbs []breadcrumb
		Paths       []path
		Channel     string
		Month       string
		Top100      bool
	}
	path struct {
		Path, Name, Icon string
	}
)

// serveDirIndex ...
func serveDirIndex(w http.ResponseWriter, base []string, paths []string) {
	tpl, err := view.GetTemplate("directory")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var dpl directoryPayload
	basePath := ""
	for _, b := range base {
		basePath += "/" + b
		dpl.Breadcrumbs = append(dpl.Breadcrumbs, breadcrumb{Path: basePath, Name: b})
	}
	if len(base) == 2 {
		dpl.Channel = base[0]
		dpl.Month = base[1]
	}
	basePath += "/"
	for _, p := range paths {
		icon := "file-text"
		if filepath.Ext(p) == "" {
			icon = "folder"
		}
		dpl.Paths = append(dpl.Paths, path{
			Path: basePath + strings.Replace(p, ".txt", "", -1),
			Name: p,
			Icon: icon,
		})
	}
	if len(dpl.Breadcrumbs) >= 2 && dpl.Breadcrumbs[1].Name != time.Now().UTC().Format("January 2006") {
		dpl.Top100 = true
	}

	w.Header().Set("Content-type", "text/html")
	if err := tpl.Execute(w, nil, dpl); err != nil {
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
	w.Header().Set("Cache-control", "max-age=60")
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
			}
		}
	}
}

// Todo
// - sort by bytes, seen, lines(default), username
// - pages????????
// TopListHandle channel index
func TopListHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	var tpl topListPayload
	tpl, err := getToplistPayload(vars["channel"], vars["month"], vars["limit"], vars["sort"])
	if err != nil {
		serveError(w, r, err)
		return
	}

	t, err := view.GetTemplate("toplist")
	if err != nil {
		serveError(w, r, errors.New("somthing went wrong getting the template :("))
		return
	}

	w.Header().Set("Content-type", "text/html")
	if err := t.Execute(w, nil, tpl); err != nil {
		serveError(w, r, errors.New("somthing went wrong :("))
		return
	}
}

func TopListApiHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	var tpl topListPayload

	tpl, err := getToplistPayload(vars["channel"], vars["month"], vars["limit"], vars["sort"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data, err := json.MarshalIndent(tpl, "", "\t")
	if err != nil {
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-type", "application/json")
	w.Write(data)
}

func getToplistPayload(channel, month, limitquery, sortquery string) (topListPayload, error) {
	var tpl topListPayload
	path := filepath.Join(common.GetConfig().LogPath, convertChannelCase(channel), month, "toplist.json.gz")

	tpl.Breadcrumbs = append(tpl.Breadcrumbs, breadcrumb{"/" + channel, channel})
	tpl.Breadcrumbs = append(tpl.Breadcrumbs, breadcrumb{"/" + channel + "/" + month, month})
	tpl.Breadcrumbs = append(tpl.Breadcrumbs, breadcrumb{"/" + channel + "/" + month + "/top" + limitquery, "Top" + limitquery})

	fi, err := os.Stat(path)
	if err != nil {
		return tpl, errors.New("check back at the end of the month :(")
	}
	tpl.Generated = fi.ModTime().UTC().Format("2006-01-02 15:04:05 MST")

	data, err := common.ReadCompressedFile(path)
	if err != nil {
		return tpl, errors.New("something went bad reading the file :(")
	}

	toplist := []*user{}

	err = gob.NewDecoder(bytes.NewBuffer(data)).Decode(&toplist)
	if err != nil {
		return tpl, errors.New("somthing went wrong :(")
	}
	tpl.MaxLimit = len(toplist) - 1

	limit := 100
	if limitquery != "" {
		limit, _ = strconv.Atoi(limitquery)
	}

	tpl.Limit = limit

	if limit > len(toplist) {
		limit = len(toplist) - 1
	}

	if len(toplist) > limit {
		toplist = toplist[:limit]
	}

	tpl.Path = "/" + channel + "/" + month

	if sortquery != "" {
		switch sortquery {
		case "bytes":
			sort.Sort(ByBytes(toplist))
		case "seen":
			sort.Sort(BySeen(toplist))
		case "username":
			sort.Sort(ByUsername(toplist))
		}
		tpl.Sort = sortquery
	}
	tpl.TopList = toplist

	for _, u := range toplist {
		u.KiloBytes = fmt.Sprintf("%.1f", float32(u.Bytes)/1024)
		if u.Seen == 0 {
			u.SeenString = "Unknown"
			continue
		}
		tm := time.Unix(u.Seen, 0).UTC()
		u.SeenString = tm.Format("2006-01-02 15:04:05 MST")
	}
	return tpl, nil
}

type (
	user struct {
		Username   string
		Lines      int
		Bytes      int
		Seen       int64
		SeenString string
		KiloBytes  string
	}
	topListPayload struct {
		Sort        string       `json:"sort"`
		Limit       int          `json:"limit"`
		MaxLimit    int          `json:"maxLimit"`
		Path        string       `json:"-"`
		Breadcrumbs []breadcrumb `json:"-"`
		Generated   string       `json:"generated"`
		TopList     []*user      `json:"topList"`
	}
	breadcrumb struct {
		Path string
		Name string
	}

	ByBytes    []*user
	BySeen     []*user
	ByUsername []*user
)

func (a ByBytes) Len() int           { return len(a) }
func (a ByBytes) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByBytes) Less(i, j int) bool { return a[i].Bytes > a[j].Bytes }

func (a BySeen) Len() int           { return len(a) }
func (a BySeen) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a BySeen) Less(i, j int) bool { return a[i].Seen < a[j].Seen }

func (a ByUsername) Len() int           { return len(a) }
func (a ByUsername) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByUsername) Less(i, j int) bool { return a[i].Username < a[j].Username }

// wip
// /stalk
// /stalk?channel=xxx&nick=xxx
func StalkerHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	channel := vars["channel"]
	nick := strings.TrimPrefix(vars["nick"], "@")

	t, err := view.GetTemplate("stalk")
	if err != nil {
		serveError(w, r, errors.New("somthing went wrong getting the template :("))
		return
	}

	var spl stalkPayload
	spl.Nick = nick
	spl.Channel = channel

	if channel == "" || nick == "" {
		if err := t.Execute(w, nil, spl); err != nil {
			serveError(w, r, errors.New("somthing went wrong 0 :("))
			return
		}
		return
	}

	path := filepath.Join(common.GetConfig().LogPath, convertChannelCase(channel))

	months, err := ioutil.ReadDir(path)
	if err != nil {
		serveError(w, r, fmt.Errorf("couldn't find channel: %s ", channel))
		return
	}

	for _, m := range months {
		if _, ok := userInMonth(convertChannelCase(channel), nick, m.Name()); ok {
			spl.Months = append(spl.Months, m.Name())
		}
	}

	if len(spl.Months) > 0 {
		sort.Sort(byMonth(spl.Months))
	} else {
		spl.Error = fmt.Sprintf("Couldn't find Nick: %s in Channel: %s :(", nick, channel)
	}

	w.Header().Set("Content-type", "text/html")
	if err := t.Execute(w, nil, spl); err != nil {
		serveError(w, r, errors.New("somthing went wrong 1 :("))
		return
	}
}

type (
	stalkPayload struct {
		Months               []string
		Nick, Channel, Error string
	}
)
