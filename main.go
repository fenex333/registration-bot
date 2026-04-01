package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"graduation-bot/bot"
	"graduation-bot/config"
	"graduation-bot/sheets"
)

func main() {
	cfg := config.Load()

	sheetsClient, err := sheets.NewClient(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize Google Sheets client: %v", err)
	}

	if err := sheetsClient.EnsureHeaders(); err != nil {
		log.Fatalf("Failed to ensure sheet headers: %v", err)
	}

	b, err := bot.New(cfg, sheetsClient)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	go sheetsClient.StartPolling(b)

	log.Println("🎓 Graduation Bot is running...")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down...")
	b.Stop()
}
