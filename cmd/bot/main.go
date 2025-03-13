package main

import (
	"log"

	"alert_bot/internal/bot"
	"alert_bot/internal/config"
)

func main() {
	cfg := config.New()

	b, err := bot.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	log.Println("Starting bot...")
	if err := b.Start(); err != nil {
		log.Fatalf("Failed to start bot: %v", err)
	}
}
