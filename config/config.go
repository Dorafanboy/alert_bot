package config

import (
	"os"
)

type Config struct {
	TelegramToken string
	Debug         bool
}

func NewConfig() *Config {
	return &Config{
		TelegramToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		Debug:         true,
	}
}
