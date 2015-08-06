package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/cloudflare/golz4"
	"github.com/gorilla/mux"
	"github.com/yosssi/ace"
)

// temp ish.. move to config
const (
	// BaseDir    = "/var/overrustle/logs"
	BaseDir    = "/var/www/public/_public"
	MaxLogSize = 10 * 1024 * 1024
	ViewPath   = "/var/overrustle/views"
)

// errors
var (
	ErrNotFound = errors.New("file not found")
)

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/", BaseHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}", ChannelHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}", MonthHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/{date:[0-9]{4}-[0-9]{2}-[0-9]{2}}.txt", DayHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/userlogs", UsersHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/userlogs/{user:[a-zA-Z0-9_-]+}.txt", UserHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/broadcaster.txt", BroadcasterHandle).Methods("GET")
	r.HandleFunc("/{channel:[a-zA-Z0-9_-]+ chatlog}/{month:[a-zA-Z]+ [0-9]{4}}/subscriptions.txt", SubHandle).Methods("GET")
	http.ListenAndServe(":8080", r)
}

// ErrorTemplate ...
func ErrorTemplate() string {
	return ""
}

// DirectoryIndex ...
func DirectoryIndex(res http.ResponseWriter, base string, paths []string) {
	tpl, err := ace.Load(ViewPath+"/layout", ViewPath+"/directory", nil)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
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
	if err := tpl.Execute(res, data); err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
}

// BaseHandle returns channel index
func BaseHandle(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-type", "text/html")

	f, err := os.Open(BaseDir)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	dirs, err := f.Readdirnames(0)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	DirectoryIndex(res, "/", dirs)
}

// ChannelHandle returns channel index
func ChannelHandle(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-type", "text/html")

	vars := mux.Vars(req)

	f, err := os.Open(BaseDir + "/" + vars["channel"])
	if err != nil {
		http.Error(res, err.Error(), http.StatusNotFound)
		return
	}

	dirs, err := f.Readdirnames(0)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	DirectoryIndex(res, "/"+vars["channel"]+"/", dirs)
}

// MonthHandle returns channel index
func MonthHandle(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-type", "text/html")

	vars := mux.Vars(req)

	f, err := os.Open(BaseDir + "/" + vars["channel"] + "/" + vars["month"])
	if err != nil {
		http.Error(res, "[]", http.StatusNotFound)
		return
	}

	files, err := f.Readdir(0)
	if err != nil {
		http.Error(res, "[]", http.StatusInternalServerError)
		return
	}

	var paths []string
	for _, file := range files {
		paths = append(paths, file.Name())
	}

	DirectoryIndex(res, "/"+vars["channel"]+"/"+vars["month"]+"/", paths)
}

// DayHandle returns channel index
func DayHandle(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-type", "text/plain")

	vars := mux.Vars(req)

	log.Println(BaseDir + "/" + vars["channel"] + "/" + vars["month"] + "/" + vars["date"] + ".txt")
	data, err := (&ChatLog{BaseDir + "/" + vars["channel"] + "/" + vars["month"] + "/" + vars["date"] + ".txt"}).Read()
	if err == ErrNotFound {
		http.Error(res, err.Error(), http.StatusNotFound)
	} else if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}

	res.Write(data)
}

// UsersHandle returns channel index
func UsersHandle(res http.ResponseWriter, req *http.Request) {

}

// UserHandle returns channel index
func UserHandle(res http.ResponseWriter, req *http.Request) {

}

// BroadcasterHandle returns channel index
func BroadcasterHandle(res http.ResponseWriter, req *http.Request) {

}

// SubHandle returns channel index
func SubHandle(res http.ResponseWriter, req *http.Request) {

}

// ChatLog file handler
type ChatLog struct {
	path string
}

func (c *ChatLog) Read() ([]byte, error) {
	var buf []byte

	f, err := os.Open(c.path + ".lz4")
	if os.IsNotExist(err) {
		f, err := os.Open(c.path)
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

func temp() {
	user := []byte("lirik:")
	baseDir := "/var/overrustle/logs/Lirik chatlog/July 2015"

	files, err := ioutil.ReadDir(baseDir)
	if err != nil {
		log.Panicf("error reading logs dir %s", err)
	}

	buf := make([]byte, 10*1024*1024)
	for _, info := range files {
		f, err := os.Open(baseDir + "/" + info.Name())
		if err != nil {
			log.Fatalf("error creating file reader %s", err)
		}

		data, _ := ioutil.ReadAll(f)

		buf = buf[:]
		lz4.Uncompress(data, buf)

		t := bytes.NewReader(buf)
		r := bufio.NewReaderSize(t, 10*1024*1024)

	ReadLine:
		for {
			line, err := r.ReadSlice('\n')
			if err == io.EOF {
				break
			} else if err != nil {
				log.Fatalf("error reading bytes %s", err)
			}

			for i := 0; i < len(user); i++ {
				if i+26 > len(line) || line[i+26] != user[i] {
					continue ReadLine
				}
			}

			fmt.Print(string(line))
		}
	}
}

func batchCompress() {
	baseDir := "/var/overrustle/logs/Lirik chatlog/July 2015"

	files, err := ioutil.ReadDir(baseDir)
	if err != nil {
		log.Panicf("error reading logs dir %s", err)
	}

	buf := make([]byte, 10*1024*1024)
	for _, info := range files {
		f, err := os.Open(baseDir + "/" + info.Name())
		if err != nil {
			log.Fatalf("error creating file reader %s", err)
		}

		data, _ := ioutil.ReadAll(f)
		size, _ := lz4.CompressHCLevel(data, buf, 16)

		err = ioutil.WriteFile(BaseDir+"/"+strings.Replace(info.Name(), ".txt", ".txt.lz4", -1), buf[:size], 0644)
		if err != nil {
			log.Println("error writing file %s", err)
		}
	}
}
