package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	TelegramToken   string
	SpreadsheetID   string
	SheetName       string
	CredentialsFile string
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, reading from environment")
	}

	cfg := &Config{
		TelegramToken:   os.Getenv("TELEGRAM_TOKEN"),
		SpreadsheetID:   os.Getenv("SPREADSHEET_ID"),
		SheetName:       os.Getenv("SHEET_NAME"),
		CredentialsFile: os.Getenv("GOOGLE_CREDENTIALS_FILE"),
	}

	if cfg.TelegramToken == "" {
		log.Fatal("TELEGRAM_TOKEN is required")
	}
	if cfg.SpreadsheetID == "" {
		log.Fatal("SPREADSHEET_ID is required")
	}
	if cfg.CredentialsFile == "" {
		cfg.CredentialsFile = "credentials.json"
	}
	if cfg.SheetName == "" {
		cfg.SheetName = "Заявки"
	}

	return cfg
}
