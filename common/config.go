package common

import (
	"encoding/json"
	"io/ioutil"
	"log"
)

// Config settings
type Config struct {
	DestinyGG struct {
		LogHost   string `json:"logHost"`
		SocketURL string `json:"socketURL"`
		OriginURL string `json:"originURL"`
		Cookie    string `json:"cookie"`
	} `json:"destinyGG"`
	Twitch struct {
		LogHost         string   `json:"logHost"`
		SocketURL       string   `json:"socketURL"`
		OriginURL       string   `json:"originURL"`
		ClientID        string   `json:"clientID"`
		ClientSecret    string   `json:"clientSecret"`
		AccessToken     string   `json:"accessToken"`
		RefreshToken    string   `json:"refreshToken"`
		OAuth           string   `json:"oAuth"`
		Nick            string   `json:"nick"`
		Admins          []string `json:"admins"`
		ChannelListPath string   `json:"channelListPath"`
		CommandChannel  string   `json:"commandChannel"`
	} `json:"twitch"`
	Server struct {
		ViewPath      string `json:"viewPath"`
		Address       string `json:"address"`
		MaxStalkLines int    `json:"maxStalkLines"`
	} `json:"server"`
	Bot struct {
		IgnoreListPath    string   `json:"ignoreListPath"`
		IgnoreLogListPath string   `json:"ignoreLogListPath"`
		Admins            []string `json:"admins"`
	} `json:"bot"`
	LogHost     string `json:"logHost"`
	LogPath     string `json:"logPath"`
	MaxOpenLogs int    `json:"maxOpenLogs"`
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

// SaveConfig ...
func SaveConfig(path string) error {
	b, err := json.MarshalIndent(&config, "", "\t")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, b, 0755)
}
