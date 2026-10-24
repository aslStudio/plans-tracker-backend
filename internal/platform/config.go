package platform

import (
	"errors"
	"fmt"
	"net"
	"os"
)

type Config struct {
	Port         string
	DatabaseURL  string
	DatabasePass string
	RedisAddr    string
	RedisPass    string
}

func loadConfig() (Config, error) {
	cfg := Config{
		Port:         os.Getenv("PORT"),
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		DatabasePass: os.Getenv("DATABASE_PASSWORD"),
		RedisAddr:    os.Getenv("REDIS_ADDR"),
		RedisPass:    os.Getenv("REDIS_PASSWORD"),
	}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	if cfg.DatabaseURL == "" || cfg.RedisAddr == "" {
		return Config{}, errors.New("DATABASE_URL and REDIS_ADDR are required")
	}
	if _, err := net.LookupPort("tcp", cfg.Port); err != nil {
		return Config{}, fmt.Errorf("invalid PORT: %w", err)
	}
	return cfg, nil
}
