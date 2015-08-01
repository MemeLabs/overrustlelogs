package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
)

// Config settings
type Config struct {
	DestinyGG struct {
		SocketURL string
		OriginURL string
		Path      string
		Premium   struct {
			Users []string
		}
	}
	Twitch struct {
		SocketURL       string
		OriginURL       string
		Path            string
		Admins          []string
		ChannelListPath string
	}
	MaxOpenLogs int
}

var config *Config

// SetupConfig loads config data from json
func SetupConfig(path string) *Config {
	config = &Config{}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatalf("invalid config path %s", path)
	}

	if err := json.Unmarshal(data, config); err != nil {
		log.Fatalf("error parsing config %s", err)
	}

	return config
}
