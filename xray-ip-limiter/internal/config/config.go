package config

import (
	"fmt"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	NATSConfig NATSConfig    `yaml:"nats"`
	Redis      RedisConfig   `yaml:"redis"`
	Service    ServiceConfig `yaml:"service"`
}

type NATSConfig struct {
	Url          string `yaml:"url" env-default:"nats://nats:4222"`
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

func MustLoad() *Config {
	configPath := "/app/config.yaml"

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		panic(fmt.Sprintf("config file not found: %s", configPath))
	}

	var cfg Config
	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		panic(fmt.Sprintf("cannot read config: %v", err))
	}

	return &cfg
}
