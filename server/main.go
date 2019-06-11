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
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/CloudyKit/jet"
	"github.com/MemeLabs/overrustlelogs/common"
	"github.com/gorilla/mux"
)

// stuff
const (
	LogLinePrefixLength = len("[2017-01-10 08:57:47 UTC] ")
	ViewsPath           = "./views"
	MaxStalkLines       = 200
)

// errors
var (
	ErrUserNotFound      = errors.New("didn't find any logs for this user")
	ErrDayNotFound       = errors.New("cou find logs for this day")
	ErrNotFound          = errors.New("file not found")
	ErrSearchKeyNotFound = errors.New("didn't find what you were looking for")
	ErrNoSubscribers     = errors.New("no subscribers for this month")
	ErrNoMentions        = errors.New("couldn't find any mentions")
)

// log file extension pattern
var (
	LogExtension   = regexp.MustCompile(`\.txt(\.gz)?$`)
	NicksExtension = regexp.MustCompile(`\.nicks\.gz$`)
	LogsPath       = "/logs"
)

// APIError ...
type APIError struct {
	Message string `json:"message"`
}

var dev = false

var view *jet.Set

func init() {
	flag.BoolVar(&dev, "dev", false, "for jet template hot reloading and local asset loading")
	flag.StringVar(&LogsPath, "logs", "/logs", "logs path for easier development")
	flag.Parse()
}

// Start server
func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	view = jet.NewHTMLSet(ViewsPath)
	view.SetDevelopmentMode(dev)
	setupViewGlobals()

	r := mux.NewRouter()
	r.Use(logger)
	r.StrictSlash(true)
	r.HandleFunc("/", BaseHandle).Methods("GET")
	r.HandleFunc("/contact", ContactHandle).Methods("GET")
	r.HandleFunc("/changelog", ChangelogHandle).Methods("GET")
	r.HandleFunc("/stalk", StalkerHandle).Methods("GET").Queries("channel", "{channel:[a-zA-Z0-9_-]+}", "nick", "{nick:@?[a-zA-Z0-9_-]+}")
	r.HandleFunc("/stalk", StalkerHandle).Methods("GET")
	r.HandleFunc("/mentions/{nick:[a-zA-Z0-9_-]{1,25}}.txt", MentionsHandle).Methods("GET").Queries("date", "{date:[0-9]{4}-[0-9]{2}-[0-9]{2}}")
	r.HandleFunc("/mentions/{nick:[a-zA-Z0-9_-]{1,25}}.txt", MentionsHandle).Methods("GET")
	r.HandleFunc("/mentions/{nick:[a-zA-Z0-9_-]{1,25}}", MentionsWrapperHandle).Methods("GET").Queries("date", "{date:[0-9]{4}-[0-9]{2}-[0-9]{2}}")
	r.HandleFunc("/mentions/{nick:[a-zA-Z0-9_-]{1,25}}", MentionsWrapperHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+}/mentions/{nick:[a-zA-Z0-9_-]{1,25}}.txt", MentionsHandle).Methods("GET").Queries("date", "{date:[0-9]{4}-[0-9]{2}-[0-9]{2}}")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+}/mentions/{nick:[a-zA-Z0-9_-]{1,25}}.txt", MentionsHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+}/mentions/{nick:[a-zA-Z0-9_-]{1,25}}", MentionsWrapperHandle).Methods("GET").Queries("date", "{date:[0-9]{4}-[0-9]{2}-[0-9]{2}}")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+}/mentions/{nick:[a-zA-Z0-9_-]{1,25}}", MentionsWrapperHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}", ChannelHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}", MonthHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/{date:[0-9]{4}-[0-9]{2}-[0-9]{2}}.txt", DayHandle).Queries("filter", "{filter:.+}").Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/{date:[0-9]{4}-[0-9]{2}-[0-9]{2}}.txt", DayHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/{date:[0-9]{4}-[0-9]{2}-[0-9]{2}}", WrapperHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/top{limit:[0-9]{1,9}}", TopListHandle).Methods("GET").Queries("sort", "{sort:[a-z]+}")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/top{limit:[0-9]{1,9}}", TopListHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/userlogs", UsersHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/userlogs/{nick:[a-zA-Z0-9_-]{1,25}}.txt", UserHandle).Queries("filter", "{filter:.+}").Methods("GET")
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
	if dev || os.Getenv("DEV") == "true" {
		r.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", http.FileServer(http.Dir("./assets"))))
	}

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
	api.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/top{limit:[0-9]{1,9}}.json", TopListAPIHandle).Methods("GET").Queries("sort", "{sort:[a-z]+}")
	api.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/top{limit:[0-9]{1,9}}.json", TopListAPIHandle).Methods("GET")

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Printf("%v", err)
		}
	}()

	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
	<-sigint
	log.Println("i love you guys, be careful")
	os.Exit(0)
}

func setupViewGlobals() {
	view.AddGlobal("title", os.Getenv("TITLE"))
	view.AddGlobal("twitter", os.Getenv("TWITTER"))
	view.AddGlobal("email", os.Getenv("SUPPORT_EMAIL"))
	view.AddGlobal("github", os.Getenv("GITHUB"))
	view.AddGlobal("donate", os.Getenv("DONATE"))
	view.AddGlobal("patreon", os.Getenv("PATREON"))
	view.AddGlobal("googleanalytics", os.Getenv("GOOGLE_ANALYTICS"))
	view.AddGlobal("googleadslot", os.Getenv("GOOGLE_AD_SLOT"))
	view.AddGlobal("googleadclient", os.Getenv("GOOGLE_AD_CLIENT"))
}

func logger(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		h.ServeHTTP(w, r)
		if strings.HasPrefix(r.URL.Path, "/assets/") || strings.HasPrefix(r.URL.Path, "/css/") || strings.HasPrefix(r.URL.Path, "/js/") {
			return
		}
		path := r.URL.Path
		if r.URL.RawQuery != "" {
			path += "?" + r.URL.RawQuery
		}
		fmt.Printf("served \"%s\" to \"%s\" in %s\n", path, r.Header.Get("Cf-Connecting-Ip"), time.Since(start))
	})
}

// NotFoundHandle channel index
func NotFoundHandle(w http.ResponseWriter, r *http.Request) {
	serveError(w, ErrNotFound)
}

// BaseHandle channel index
func BaseHandle(w http.ResponseWriter, r *http.Request) {
	paths, err := readDirIndex(LogsPath)
	if err != nil {
		serveError(w, err)
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

	crumbs := strings.Split(r.URL.Path, "/")
	var bc []breadcrumb
	basePath := ""
	for _, b := range crumbs {
		if b == "" {
			continue
		}
		basePath += "/" + b
		bc = append(bc, breadcrumb{Path: basePath, Name: b})
	}

	w.Header().Set("Content-type", "text/html; charset=UTF-8")
	path := r.URL.Path + ".txt"
	if r.URL.RawQuery != "" {
		path += "?" + r.URL.RawQuery
	}
	if err := tpl.Execute(w, nil, struct {
		Path        string
		Breadcrumbs []breadcrumb
	}{Path: path, Breadcrumbs: bc}); err != nil {
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
	paths, err := readDirIndex(filepath.Join(LogsPath, vars["channel"]))
	if err != nil {
		serveError(w, err)
		return
	}
	sort.Sort(byMonth(paths))
	serveDirIndex(w, []string{vars["channel"]}, paths)
}

// MonthHandle channel index
func MonthHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	paths, err := readLogDir(filepath.Join(LogsPath, convertChannelCase(vars["channel"]), vars["month"]))
	if err != nil {
		serveError(w, err)
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
	data, err := readLogFile(filepath.Join(LogsPath, convertChannelCase(vars["channel"]), vars["month"], vars["date"]))
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
			_, _ = w.Write(line)
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
	f, err := os.Open(filepath.Join(LogsPath, convertChannelCase(vars["channel"]), vars["month"]))
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
			_ = common.ReadNickList(nicks, filepath.Join(LogsPath, convertChannelCase(vars["channel"]), vars["month"], file.Name()))
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
		serveFilteredLogs(w, filepath.Join(LogsPath, vars["channel"], vars["month"]), searchKey(nick, vars["filter"]))
		return
	}
	serveFilteredLogs(w, filepath.Join(LogsPath, vars["channel"], vars["month"]), nickFilter(nick))
}

func userInMonth(channel, nick, month string) (string, bool) {
	search, err := common.NewNickSearch(filepath.Join(LogsPath, channel), nick)
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
	serveFilteredLogs(w, filepath.Join(LogsPath, vars["channel"], vars["month"]), nickFilter(nick))
}

// SubscriberHandle channel index
func SubscriberHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	vars["channel"] = convertChannelCase(vars["channel"])
	nick, ok := userInMonth(vars["channel"], "twitchnotify", vars["month"])
	if !ok {
		http.Error(w, ErrNoSubscribers.Error(), http.StatusInternalServerError)
		return
	}
	serveFilteredLogs(w, filepath.Join(LogsPath, vars["channel"], vars["month"]), nickFilter(nick))
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
	serveFilteredLogs(w, filepath.Join(LogsPath, vars["channel"], vars["month"]), nickFilter(nick))
}

// DestinySubscriberHandle destiny subscriber logs
func DestinySubscriberHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	vars["channel"] = "Destinygg chatlog"
	nick, ok := userInMonth(vars["channel"], "Subscriber", vars["month"])
	if !ok {
		http.Error(w, ErrNoSubscribers.Error(), http.StatusInternalServerError)
		return
	}
	serveFilteredLogs(w, filepath.Join(LogsPath, vars["channel"], vars["month"]), nickFilter(nick))
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
	serveFilteredLogs(w, filepath.Join(LogsPath, vars["channel"], vars["month"]), nickFilter(nick))
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
	search, err := common.NewNickSearch(filepath.Join(LogsPath, vars["channel"]), vars["nick"])
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

// MentionsWrapperPayload payload for mentions.jet
type MentionsWrapperPayload struct {
	Days []*MentionsDay
}

// MentionsDay part of the payload for mentions.jet
type MentionsDay struct {
	Name string
	Log  string
}

// MentionsWrapperHandle ...
func MentionsWrapperHandle(w http.ResponseWriter, r *http.Request) {
	tpl, err := view.GetTemplate("mentions")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	vars := mux.Vars(r)
	if _, ok := vars["channel"]; ok {
		vars["channel"] = strings.Title(strings.ToLower(vars["channel"])) + " chatlog"
	} else {
		vars["channel"] = "Destinygg chatlog"
	}

	if _, ok := vars["date"]; !ok {
		vars["date"] = time.Now().UTC().Format("2006-01-02")
	}
	date, err := time.Parse("2006-01-02", vars["date"])
	if err != nil {
		serveError(w, errors.New("invalid date format"))
		return
	}
	days := []time.Time{
		date,
		date.AddDate(0, 0, -1),
		date.AddDate(0, 0, -2),
		date.AddDate(0, 0, -3),
	}

	var payload MentionsWrapperPayload

	for _, day := range days {
		if day.After(time.Now().UTC()) {
			continue
		}
		d := MentionsDay{
			Name: day.Format("2006-01-02"),
		}
		payload.Days = append(payload.Days, &d)
		data, err := readLogFile(filepath.Join(LogsPath, convertChannelCase(vars["channel"]), day.Format("January 2006"), day.Format("2006-01-02")))
		if err != nil {
			d.Log = err.Error()
			continue
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
			lowerLine := bytes.ToLower(line)
			if isMentioned([]byte(" "+vars["nick"]), lowerLine) {
				d.Log += string(line)
				lineCount++
			}
		}
		if lineCount == 0 {
			d.Log = ErrNoMentions.Error()
		}
	}

	w.Header().Set("Content-type", "text/html")
	if err := tpl.Execute(w, nil, payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
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
		http.Error(w, "can't look into the future", http.StatusNotFound)
		return
	}
	data, err := readLogFile(filepath.Join(LogsPath, convertChannelCase(vars["channel"]), t.Format("January 2006"), t.Format("2006-01-02")))
	if err != nil {
		http.Error(w, ErrDayNotFound.Error(), http.StatusNotFound)
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
			_, _ = w.Write(line)
			lineCount++
		}
	}
	if lineCount == 0 {
		http.Error(w, ErrNoMentions.Error(), http.StatusNotFound)
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

	if _, ok := vars["date"]; !ok {
		vars["date"] = time.Now().UTC().Format("2006-01-02")
	}
	t, err := time.Parse("2006-01-02", vars["date"])
	if err != nil {
		serveAPIError(w, "invalid date format", http.StatusBadRequest)
		return
	}
	if t.After(time.Now().UTC()) {
		serveAPIError(w, "can't look into the future", http.StatusBadRequest)
		return
	}

	data, err := readLogFile(filepath.Join(LogsPath, convertChannelCase(vars["channel"]), t.Format("January 2006"), t.Format("2006-01-02")))
	if err != nil {
		serveAPIError(w, ErrDayNotFound.Error(), http.StatusNotFound)
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
		serveAPIError(w, ErrNoMentions.Error(), http.StatusNotFound)
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
			serveAPIError(w, "limit query is not a integer", http.StatusBadRequest)
			return
		}
		limit = l
	}

	var buf = lines
	if len(lines)-limit > 0 {
		buf = lines[len(lines)-limit:]
	}

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
	_ = json.NewEncoder(w).Encode(mentions)
}

// ChannelsAPIHandle lists the channels
func ChannelsAPIHandle(w http.ResponseWriter, r *http.Request) {

	files, err := readDirIndex(LogsPath)
	if err != nil {
		serveAPIError(w, err.Error(), http.StatusNotFound)
		return
	}

	for i, v := range files {
		files[i] = v[:len(v)-8]
	}
	sort.Strings(files)
	w.Header().Set("Content-type", "application/json")
	_ = json.NewEncoder(w).Encode(files)
}

// MonthsAPIHandle lists the channels
func MonthsAPIHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	monthsPath := filepath.Join(LogsPath, strings.Title(strings.ToLower(vars["channel"]))+" chatlog")
	files, err := readDirIndex(monthsPath)
	if err != nil {
		serveAPIError(w, err.Error(), http.StatusNotFound)
		return
	}

	sort.Sort(byMonth(files))
	w.Header().Set("Content-type", "application/json")
	_ = json.NewEncoder(w).Encode(files)
}

// DaysAPIHandle lists the channels
func DaysAPIHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	daysPath := filepath.Join(LogsPath, strings.Title(strings.ToLower(vars["channel"]))+" chatlog", vars["month"])
	files, err := readDirIndex(daysPath)
	if err != nil {
		serveAPIError(w, err.Error(), http.StatusNotFound)
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

	w.Header().Set("Content-type", "application/json")
	_ = json.NewEncoder(w).Encode(filteredDirs)
}

// LinesAPIHandle lists the channels
func LinesAPIHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	monthPath := filepath.Join(LogsPath, strings.Title(strings.ToLower(vars["channel"]))+" chatlog", vars["month"])
	files, err := readDirIndex(monthPath)
	if err != nil {
		serveAPIError(w, err.Error(), http.StatusNotFound)
		return
	}
	var temp linesData
	for _, v := range files {
		if strings.Contains(v, ".nicks") {
			continue
		}
		if strings.Contains(v, ".txt.gz") {
			b, err := readLogFile(filepath.Join(monthPath, v))
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

	w.Header().Set("Content-type", "application/json")
	_ = json.NewEncoder(w).Encode(temp)
}

// ByDate ...
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
	usersPath := filepath.Join(LogsPath, strings.Title(strings.ToLower(vars["channel"]))+" chatlog", vars["month"])
	files, err := readDirIndex(usersPath)
	if err != nil {
		serveAPIError(w, err.Error(), http.StatusNotFound)
		return
	}
	nicks := common.NickList{}
	for _, file := range files {
		if NicksExtension.MatchString(file) {
			_ = common.ReadNickList(nicks, filepath.Join(LogsPath, strings.Title(strings.ToLower(vars["channel"]))+" chatlog", vars["month"], file))
		}
	}
	names := make([]string, 0, len(nicks))
	for nick := range nicks {
		names = append(names, nick+".txt")
	}
	sort.Strings(names)

	w.Header().Set("Content-type", "application/json")
	_ = json.NewEncoder(w).Encode(names)
}

// StalkHandle return n most recent lines of chat for user
func StalkHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	if _, ok := vars["limit"]; !ok {
		vars["limit"] = "3"
	}
	vars["channel"] = strings.Title(strings.ToLower(vars["channel"])) + " chatlog"
	limit, err := strconv.ParseUint(vars["limit"], 10, 32)
	if err != nil {
		serveAPIError(w, "failed parsing limit", http.StatusBadRequest)
		return
	}
	if limit > uint64(MaxStalkLines) {
		limit = uint64(MaxStalkLines)
	} else if limit < 1 {
		limit = 3
	}
	buf := make([]string, limit)
	index := limit
	search, err := common.NewNickSearch(filepath.Join(LogsPath, vars["channel"]), vars["nick"])
	if err != nil {
		serveAPIError(w, err.Error(), http.StatusNotFound)
		return
	}

ScanLogs:
	for {
		rs, err := search.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			serveAPIError(w, err.Error(), http.StatusNotFound)
			return
		}
		data, err := readLogFile(filepath.Join(LogsPath, convertChannelCase(vars["channel"]), rs.Month(), rs.Day()))
		if err != nil {
			serveAPIError(w, err.Error(), http.StatusNotFound)
			return
		}
		var lines [][]byte
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
		serveAPIError(w, ErrUserNotFound.Error(), http.StatusNotFound)
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
	data.Nick = strings.ToLower(vars["nick"])
	for i := int(index); i < len(buf); i++ {
		t, err := time.Parse("2006-01-02 15:04:05 MST", buf[i][1:24])
		if err != nil {
			continue
		}
		ci := strings.Index(buf[i][LogLinePrefixLength:], ":")
		data.Lines = append(data.Lines, Line{
			Timestamp: t.Unix(),
			Text:      buf[i][ci+LogLinePrefixLength+2:],
		})
	}
	w.Header().Set("Content-type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
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
func serveError(w http.ResponseWriter, e error) {
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
		e = errors.New("unknown Error")
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
		icon := "file-alt"
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
				_, _ = w.Write(line)
			}
		}
	}
}

// serveAPIError servers a error with given message and status code
func serveAPIError(w http.ResponseWriter, error string, code int) {
	apiError := APIError{Message: error}
	w.WriteHeader(code)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(apiError)
}

// TopListHandle channel index
// - sort by bytes, seen, lines(default), username
// - pages????????
func TopListHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	var tpl topListPayload
	tpl, err := getToplistPayload(vars["channel"], vars["month"], vars["limit"], vars["sort"])
	if err != nil {
		serveError(w, err)
		return
	}

	t, err := view.GetTemplate("toplist")
	if err != nil {
		serveError(w, errors.New("failed loading toplist template"))
		return
	}

	w.Header().Set("Content-type", "text/html")
	if err := t.Execute(w, nil, tpl); err != nil {
		serveError(w, errors.New("failed executing toplist template"))
		return
	}
}

// TopListAPIHandle ...
func TopListAPIHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	var tpl topListPayload

	tpl, err := getToplistPayload(vars["channel"], vars["month"], vars["limit"], vars["sort"])
	if err != nil {
		serveAPIError(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-type", "application/json")
	_ = json.NewEncoder(w).Encode(tpl)
}

func getToplistPayload(channel, month, limitquery, sortquery string) (topListPayload, error) {
	var tpl topListPayload
	path := filepath.Join(LogsPath, convertChannelCase(channel), month, "toplist.json.gz")

	tpl.Breadcrumbs = append(tpl.Breadcrumbs, breadcrumb{"/" + channel, channel})
	tpl.Breadcrumbs = append(tpl.Breadcrumbs, breadcrumb{"/" + channel + "/" + month, month})
	tpl.Breadcrumbs = append(tpl.Breadcrumbs, breadcrumb{"/" + channel + "/" + month + "/top" + limitquery, "Top" + limitquery})

	fi, err := os.Stat(path)
	if err != nil {
		return tpl, errors.New("check back at the end of the month")
	}
	tpl.Generated = fi.ModTime().UTC().Format("2006-01-02 15:04:05 MST")

	data, err := common.ReadCompressedFile(path)
	if err != nil {
		return tpl, errors.New("failed reading toplist file")
	}

	var toplist []*user
	err = gob.NewDecoder(bytes.NewBuffer(data)).Decode(&toplist)
	if err != nil {
		return tpl, errors.New("failed decoding toplist file")
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
			sort.Sort(byBytes(toplist))
		case "seen":
			sort.Sort(bySeen(toplist))
		case "username":
			sort.Sort(byUsername(toplist))
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

	byBytes    []*user
	bySeen     []*user
	byUsername []*user
)

func (a byBytes) Len() int           { return len(a) }
func (a byBytes) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byBytes) Less(i, j int) bool { return a[i].Bytes > a[j].Bytes }

func (a bySeen) Len() int           { return len(a) }
func (a bySeen) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a bySeen) Less(i, j int) bool { return a[i].Seen < a[j].Seen }

func (a byUsername) Len() int           { return len(a) }
func (a byUsername) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byUsername) Less(i, j int) bool { return a[i].Username < a[j].Username }

// StalkerHandle ...
func StalkerHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	channel := strings.TrimSpace(vars["channel"])
	nick := strings.TrimSpace(strings.TrimPrefix(vars["nick"], "@"))

	t, err := view.GetTemplate("stalk")
	if err != nil {
		serveError(w, errors.New("failed loading stalk template"))
		return
	}

	var spl stalkPayload
	spl.Nick = nick
	spl.Channel = channel

	if nick == "" || channel == "" {
		if err := t.Execute(w, nil, spl); err != nil {
			serveError(w, errors.New("failed executing stalk template"))
		}
		return
	}

	path := filepath.Join(LogsPath, convertChannelCase(channel))

	months, err := readDirIndex(path)
	if err != nil {
		serveError(w, fmt.Errorf("couldn't find channel: %s ", channel))
		return
	}

	workers := runtime.NumCPU()
	monthChan := make(chan string, len(months))
	var wg sync.WaitGroup
	wg.Add(workers)
	var monthMutex sync.Mutex
	for i := 0; i < workers; i++ {
		go func() {
			for m := range monthChan {
				if _, ok := userInMonth(convertChannelCase(channel), nick, m); ok {
					monthMutex.Lock()
					spl.Months = append(spl.Months, m)
					monthMutex.Unlock()
				}
			}
			wg.Done()
		}()
	}
	for _, m := range months {
		monthChan <- m
	}
	close(monthChan)
	wg.Wait()

	if len(spl.Months) > 0 {
		sort.Sort(byMonth(spl.Months))
	} else {
		spl.Error = fmt.Sprintf("Couldn't find Nick: %s in Channel: %s", nick, channel)
	}

	w.Header().Set("Content-type", "text/html")
	if err := t.Execute(w, nil, spl); err != nil {
		serveError(w, errors.New("failed executing stalk template"))
	}
}

type (
	stalkPayload struct {
		Months               []string
		Nick, Channel, Error string
	}
)
