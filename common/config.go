package common

import (
	"encoding/json"
	"io/ioutil"
	"log"
)

// Config settings
type Config struct {
	DestinyGG struct {
		LogHost   string
		SocketURL string
		OriginURL string
		Cookie    string
		Premium   struct {
			Users []string
		}
	}
	Twitch struct {
		SocketURL       string
		OriginURL       string
		OAuth           string
		Nick            string
		Admins          []string
		ChannelListPath string
	}
	Server struct {
		ViewPath      string
		Address       string
		MaxStalkLines int
	}
	Bot struct {
		IgnoreListPath string
		Admins         []string
	}
	LogHost     string
	LogPath     string
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

// GetConfig returns config
func GetConfig() *Config {
	return config
}
