package main

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"

	"github.com/cloudflare/golz4"
	"github.com/gorilla/mux"
	"github.com/xlab/handysort"
	"github.com/yosssi/ace"
)

// temp ish.. move to config
const (
	// BaseDir    = "/var/overrustle/logs"
	BaseDir             = "/var/www/public/_public"
	MaxLogSize          = 10 * 1024 * 1024
	ViewPath            = "/var/overrustle/views"
	MaxUserNameLength   = 25
	LogLinePrefixLength = 26
)

// errors
var (
	ErrNotFound = errors.New("file not found")
)

// log file extension pattern
var (
	LogExtension = regexp.MustCompile("\\.txt(\\.lz4)?$")
)

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/", BaseHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}", ChannelHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}", MonthHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/{date:[0-9]{4}-[0-9]{2}-[0-9]{2}}.txt", DayHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/userlogs", UsersHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/userlogs/{user:[a-zA-Z0-9_-]+}.txt", UserHandle).Methods("GET")
	r.HandleFunc("/Destinygg chatlog/{month:[a-zA-Z]+ [0-9]{4}}/broadcaster.txt", DestinyHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/broadcaster.txt", BroadcasterHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/subscribers.txt", SubHandle).Methods("GET")
	http.ListenAndServe(":8080", r)
}

// BaseHandle returns channel index
func BaseHandle(w http.ResponseWriter, r *http.Request) {
	paths, err := readDirIndex(BaseDir)
	if err != nil {
		serveError(w, err)
		return
	}
	serveDirIndex(w, "/", paths)
}

// ChannelHandle returns channel index
func ChannelHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	paths, err := readDirIndex(BaseDir + "/" + vars["channel"])
	if err != nil {
		serveError(w, err)
		return
	}
	serveDirIndex(w, "/"+vars["channel"]+"/", paths)
}

// MonthHandle returns channel index
func MonthHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	paths, err := readLogDir(BaseDir + "/" + vars["channel"] + "/" + vars["month"])
	if err != nil {
		serveError(w, err)
		return
	}
	paths = append(paths, make([]string, 2)...)
	copy(paths[2:], paths)
	paths[0] = "broadcaster.txt"
	paths[1] = "subscribers.txt"
	for i, path := range paths {
		paths[i] = LogExtension.ReplaceAllString(path, ".txt")
	}
	serveDirIndex(w, "/"+vars["channel"]+"/"+vars["month"]+"/", paths)
}

// DayHandle returns channel index
func DayHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	data, err := readLogFile(BaseDir + "/" + vars["channel"] + "/" + vars["month"] + "/" + vars["date"])
	if err != nil {
		serveError(w, err)
		return
	}
	w.Header().Set("Content-type", "text/plain")
	w.Write(data)
}

// UsersHandle returns channel index
func UsersHandle(w http.ResponseWriter, r *http.Request) {

}

// UserHandle returns channel index
func UserHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serveUserLog(w, BaseDir+"/"+vars["channel"]+"/"+vars["month"], vars["user"])
}

// BroadcasterHandle returns channel index
func BroadcasterHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serveUserLog(w, BaseDir+"/"+vars["channel"]+"/"+vars["month"], vars["channel"][:len(vars["channel"])-8])
}

// DestinyHandle returns channel index
func DestinyHandle(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serveUserLog(w, BaseDir+"/Destinygg chatlog/"+vars["month"], "Destiny")
}

// SubHandle returns channel index
func SubHandle(w http.ResponseWriter, r *http.Request) {

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
	f, err := os.Open(path + ".txt.lz4")
	if os.IsNotExist(err) {
		f, err := os.Open(path + ".txt")
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		buf, err = ioutil.ReadAll(f)
		if err != nil {
			return nil, err
		}
	} else {
		if err != nil {
			return nil, ErrNotFound
		}
		buf = make([]byte, MaxLogSize)
		data, err := ioutil.ReadAll(f)
		if err != nil {
			return nil, err
		}
		if err := lz4.Uncompress(data, buf); err != nil {
			return nil, err
		}
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
	tpl, err := ace.Load(ViewPath+"/layout", ViewPath+"/directory", nil)
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
	if len(user) > MaxUserNameLength {
		serveError(w, ErrNotFound)
	}
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
		}
		t := bytes.NewReader(data)
		r := bufio.NewReaderSize(t, MaxLogSize)

		for {
			line, err := r.ReadSlice('\n')
			if err == io.EOF {
				break
			} else if err != nil {
				log.Fatalf("error reading bytes %s", err)
			}
			if filter(line) {
				w.Write(line)
			}
		}
	}
}
