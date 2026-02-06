package config

import (
	"os"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Bot struct {
		Name   string  `yaml:"NAME"`
		Token  string  `yaml:"TOKEN"`
		Admins []int64 `yaml:"ADMINS"`
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
	for _, admin := range c.Bot.Admins {
		if admin == userID {
			return true
		}
	}
	return false
}
