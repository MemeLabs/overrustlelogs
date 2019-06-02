package common

import (
	"log"

	"github.com/BurntSushi/toml"
)

// Config settings
type Config struct {
	DestinyGG struct {
		LogHost   string `toml:"logHost"`
		SocketURL string `toml:"socketURL"`
		OriginURL string `toml:"originURL"`
		Cookie    string `toml:"cookie"`
	} `toml:"destinyGG"`
	Twitch struct {
		LogHost        string   `toml:"logHost"`
		SocketURL      string   `toml:"socketURL"`
		OriginURL      string   `toml:"originURL"`
		ClientID       string   `toml:"clientID"`
		OAuth          string   `toml:"oAuth"`
		Nick           string   `toml:"nick"`
		Admins         []string `toml:"admins"`
		CommandChannel string   `toml:"commandChannel"`
	} `toml:"twitch"`
	Bot struct {
		Admins []string `toml:"admins"`
	} `toml:"bot"`
	LogHost     string `toml:"logHost"`
	MaxOpenLogs int    `toml:"maxOpenLogs"`
}

var config *Config

// SetupConfig loads config data from json
func SetupConfig(path string) *Config {
	config = &Config{}

	_, err := toml.DecodeFile(path, &config)
	if err != nil {
		log.Fatalf("error parsing config, err : %v", err)
	}

	return config
}

// GetConfig returns config
func GetConfig() *Config {
	return config
}
