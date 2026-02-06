package config

import (
	"log"
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
		log.Fatalf("读取配置文件失败: %v", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(f, &cfg); err != nil {
		log.Fatalf("解析配置文件失败: %v", err)
	}

	if cfg.Bot.Token == "" {
		log.Fatal("配置文件错误: BOT/TOKEN 不能为空")
	}
	if cfg.Bot.Admin == "" {
		log.Fatal("配置文件错误: BOT/ADMINS 不能为空")
	}

	return &cfg
}

func (c *Config) IsAdmin(userID int64) bool {
	return strconv.FormatInt(userID, 10) == c.Bot.Admin
}
