package config

import (
	"fmt"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	NATS     NATSConfig     `yaml:"nats"`
	Redis    RedisConfig    `yaml:"redis"`
	Service  ServiceConfig  `yaml:"service"`
	Telegram TelegramConfig `yaml:"telegram"`
}

type NATSConfig struct {
	URL          string `yaml:"url" env-default:"nats://nats:4222"`
	Token        string `yaml:"token" env-required:"true"`
	WorkersCount int    `yaml:"workers_count" env-default:"3"`
}

type RedisConfig struct {
	Addr     string `yaml:"addr" env-default:"redis:6379"`
	Password string `yaml:"password" env-default:""`
	DB       int    `yaml:"db" env-default:"0"`
}

type ServiceConfig struct {
	IPLimit     int           `yaml:"ip_limit" env-default:"2"`
	BanDuration time.Duration `yaml:"ban_duration" env-default:"1m"`
}

type TelegramConfig struct {
	Enabled  bool   `yaml:"enabled" env-default:"false"`
	BotToken string `yaml:"bot_token"`
	ChatID   string `yaml:"chat_id"`
}

func MustLoad() *Config {
	configPath := getConfigPath()

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		panic(fmt.Sprintf("config file not found: %s", configPath))
	}

	var cfg Config
	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		panic(fmt.Sprintf("cannot read config: %v", err))
	}

	return &cfg
}

func getConfigPath() string {
	if path := os.Getenv("CONFIG_PATH"); path != "" {
		return path
	}
	return "/app/config.yaml"
}
