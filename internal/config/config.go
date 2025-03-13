package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	TelegramToken   string
	Debug           bool
	ReminderTime    time.Duration
	Timezone        string
	ReminderTopicID map[int64]int // Мапа для хранения ID топиков для каждого чата
}

func New() *Config {
	// Загружаем .env файл
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found: %v", err)
	}

	// Читаем токен бота
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN is required. Please set it in .env file or environment variables")
	}

	// Читаем режим отладки
	debug := false
	if debugStr := os.Getenv("BOT_DEBUG"); debugStr != "" {
		debug, _ = strconv.ParseBool(debugStr)
	}

	// Читаем время напоминания (в минутах)
	reminderMinutes := 10
	if reminderStr := os.Getenv("REMINDER_MINUTES"); reminderStr != "" {
		if minutes, err := strconv.Atoi(reminderStr); err == nil {
			reminderMinutes = minutes
		}
	}

	// Читаем часовой пояс
	timezone := "Europe/Moscow"
	if tz := os.Getenv("TZ"); tz != "" {
		timezone = tz
	}

	return &Config{
		TelegramToken:   token,
		Debug:           debug,
		ReminderTime:    time.Duration(reminderMinutes) * time.Minute,
		Timezone:        timezone,
		ReminderTopicID: make(map[int64]int),
	}
}
