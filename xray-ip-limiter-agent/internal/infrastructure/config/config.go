package config

import (
	"fmt"
	"os"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	NodeID string `yaml:"node_id"`

	LogPath string `yaml:"log_path"`

	NATS NATSConfig `yaml:"nats"`

	Debug DebugConfig `yaml:"debug"`
}

type NATSConfig struct {
	URL   string `yaml:"url" env:"NATS_URL"`
	Token string `yaml:"token" env:"NATS_TOKEN"`
}

type DebugConfig struct {
	DisableBlock bool `yaml:"disable_block"`
	DebugLog     bool `yaml:"debug_log"`
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
