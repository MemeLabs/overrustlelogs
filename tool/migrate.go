package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"time"

	"github.com/slugalisk/overrustlelogs/common"
	"github.com/xlab/handysort"
)

var (
	logLine      = regexp.MustCompile("[\\s\\r\\n]*\\[?(.+?)\\] ?[^a-zA-Z0-9_]*?([a-zA-Z0-9_]+|##################################)[^a-zA-Z0-9_]*?\\:? (.*)(?:[\\r\\n]+?|$)")
	metaLine     = regexp.MustCompile("[\\s\\r\\n]*\\[?(.+?)\\] ?(.*)(?:[\\r\\n]+?|$)")
	fileNameDate = regexp.MustCompile("([0-9]+-[0-9]+-[0-9]+)")
	timeFormats  = []struct {
		format    string
		ambiguous bool
	}{
		{"2006-01-02 15:04:05 MST", false},
		{"Jan 2 2006 15:04:05 MST", false},
		{"Jan 2 2006 15:04:05", false},
		{"01/02/2006 3:04:05 PM", false},
		{"01/02/2006 15:04:05", false},
		{"01/02/2006 15:04:05 MST", false},
		{"2006/01/02 15:04:05 MST", false},
		{"Mon Jan 02 2006 15:04:05 UTC", false},
		{"Jan 2 15:04:05 MST", true},
		{"3:04:05 PM", true},
	}
	dateFormats = []string{
		"01-02-2006",
		"2006-01-02",
	}
	timeFormat = "2006-01-02 15:04:05 MST"
	dateFormat = "2006-01-02"
)

// errors
var (
	ErrLineNotFound = errors.New("line not found in input")
	ErrGarbageData  = errors.New("garbage data in input preceding line")
)

func migrate() error {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	if len(os.Args) < 4 {
		return errors.New("not enough args")
	}
	src := os.Args[2]
	dst := os.Args[3]

	f, err := os.Open(src)
	if err != nil {
		return err
	}
	channels, err := f.Readdirnames(0)
	if err != nil {
		return err
	}
	for _, c := range channels {
		f, err := os.Open(src + "/" + c)
		if err != nil {
			log.Println(err)
			continue
		}
		months, err := f.Readdirnames(0)
		if err != nil {
			log.Println(err)
			continue
		}
		for _, m := range months {
			f, err := os.Open(src + "/" + c + "/" + m)
			if err != nil {
				log.Println(err)
				continue
			}
			logs, err := f.Readdir(0)
			if err != nil {
				log.Println(err)
				continue
			}
			banInjector, err := newInjector("Ban", src+"/"+c+"/"+m+"/bans.txt")
			if err != nil && !os.IsNotExist(err) {
				log.Printf("error reading bans %s", err)
			}
			subInjector, err := newInjector("Subscriber", src+"/"+c+"/"+m+"/subs.txt")
			if err != nil && !os.IsNotExist(err) {
				log.Printf("error reading subs %s", err)
			}
			names := make([]string, len(logs))
			for i, l := range logs {
				names[i] = l.Name()
			}
			sort.Sort(handysort.Strings(names))
			for _, l := range names {
				if d := fileNameDate.FindString(l); d != "" {
					if d, err = normalizeDate(d); err != nil {
						continue
					}
					srcFile := src + "/" + c + "/" + m + "/" + l
					dstFile := dst + "/" + c + "/" + m + "/" + d + ".txt"
					data, err := ioutil.ReadFile(srcFile)
					if err != nil {
						continue
					}
					if _, err := os.Stat(dst + "/" + c + "/" + m); err != nil {
						err := os.MkdirAll(dst+"/"+c+"/"+m, 0755)
						if err != nil {
							log.Printf("error creating target dir %s", err)
							continue
						}
					}
					f, err := os.OpenFile(dstFile, os.O_CREATE|os.O_TRUNC|os.O_APPEND|os.O_WRONLY, 0644)
					if err != nil {
						log.Printf("error creating target file %s", err)
						continue
					}
					for {
						parts, err := readLine(&data, logLine)
						if err != nil {
							if err != io.EOF {
								log.Printf("error reading log line %s %s", srcFile, err)
							}
							break
						}
						t, err := parseTime(d, parts[0])
						if err != nil {
							log.Printf("error parsing time %s \"%s\" %s", srcFile, parts[0], err)
						}
						if banInjector != nil && banInjector.currentTime != nil && t.After(*banInjector.currentTime) {
							if _, err := f.WriteString(banInjector.currentLine); err != nil {
								log.Printf("error writing log line %s", err)
								break
							}
							if err := banInjector.advance(); err != nil {
								log.Printf("error advancing ban injector %s", err)
							}
						}
						if subInjector != nil && subInjector.currentTime != nil && t.After(*subInjector.currentTime) {
							if _, err := f.WriteString(subInjector.currentLine); err != nil {
								log.Printf("error writing log line %s", err)
								break
							}
							if err := subInjector.advance(); err != nil {
								log.Printf("error advancing sub injector %s", err)
							}
						}
						if parts[1] == "##################################" {
							parts[1] = "twitchnotify"
						}
						if _, err := f.WriteString(formatLine(t, parts[1], parts[2])); err != nil {
							log.Printf("error writing log line %s", err)
							break
						}
					}
					f.Close()
					go func() {
						c := exec.Command(os.Args[0], "nicks", dstFile)
						if err := c.Run(); err != nil {
							log.Printf("error generating nick list for %s %s", dstFile, err)
						}
						common.CompressFile(dstFile)
					}()
					log.Printf("finished with %s", srcFile)
				}
			}
		}
	}
	return nil
}

type logInjector struct {
	nick        string
	currentTime *time.Time
	currentLine string
	data        []byte
}

func newInjector(nick string, path string) (*logInjector, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	i := &logInjector{
		nick: nick,
		data: data,
	}
	if err := i.advance(); err != nil {
		return nil, err
	}
	return i, nil
}

func (i *logInjector) advance() error {
	parts, err := readLine(&i.data, metaLine)
	if err != nil {
		return err
	}
	if i.currentTime, err = parseTime("", parts[0]); err != nil {
		return err
	}
	i.currentLine = formatLine(i.currentTime, i.nick, parts[1])
	return nil
}

func readLine(data *[]byte, pattern *regexp.Regexp) ([]string, error) {
	if len(*data) == 0 {
		return nil, io.EOF
	}
	indexes := pattern.FindSubmatchIndex(*data)
	if indexes == nil {
		log.Println(len(*data), string((*data)[:200]))
		return nil, ErrLineNotFound
	} else if indexes[0] != 0 {
		log.Println(indexes)
		fmt.Println(string((*data)[:indexes[len(indexes)-1]]))
		return nil, ErrGarbageData
	}
	parts := make([]string, len(indexes)/2-1)
	for i := 2; i < len(indexes); i += 2 {
		parts[i/2-1] = string((*data)[indexes[i]:indexes[i+1]])
	}
	*data = (*data)[indexes[1]:]
	return parts, nil
}

func formatLine(t *time.Time, n string, d string) string {
	return "[" + t.Format(timeFormat) + "] " + n + ": " + d + "\n"
}

func parseTime(b string, d string) (*time.Time, error) {
	var t time.Time
	var err error
	for _, f := range timeFormats {
		if t, err = time.Parse(f.format, d); err == nil {
			if f.ambiguous {
				t, err = time.Parse(dateFormat+" "+f.format, b+" "+d)
			}
			break
		}
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func normalizeTime(b string, d string) (string, error) {
	t, err := parseTime(b, d)
	if err != nil {
		return "", err
	}
	return t.Format("2006-01-02 15:04:05 MST"), nil
}

func normalizeDate(d string) (string, error) {
	var t time.Time
	var err error
	for _, f := range dateFormats {
		if t, err = time.Parse(f, d); err == nil {
			break
		}
	}
	if err != nil {
		return "", err
	}
	return t.Format("2006-01-02"), nil
}
