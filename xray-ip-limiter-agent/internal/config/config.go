package config

import (
	"fmt"
	"os"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	NodeID  string `yaml:"node_id"`
	LogPath string `yaml:"log_path"`

	NATS NATSConfig `yaml:"nats"`
}

type NATSConfig struct {
	Url   string `yaml:"url"`
	Token string `yaml:"token"`
}

func MustLoad() *Config {
	configPath := "/app/config.yaml"

	if configPath == "" {
		panic("not found config file path")
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Println(configPath)
		panic("not found config file")
	}

	var cfg Config
	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		panic("cannot read config")
	}

	return &cfg
}
