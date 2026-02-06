package config

import (
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Bot struct {
		Name  string `yaml:"NAME"`
		Token string `yaml:"TOKEN"`
		Admin string `yaml:"ADMINS"`
	} `yaml:"BOT"`
}

func Load() *Config {
	f, err := os.ReadFile("config.yaml")
	if err != nil {
		panic(err)
	}
	var cfg Config
	if err := yaml.Unmarshal(f, &cfg); err != nil {
		panic(err)
	}
	return &cfg
}

func (c *Config) IsAdmin(userID int64) bool {
	return strconv.FormatInt(userID, 10) == c.Bot.Admin
}
