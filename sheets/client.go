package sheets

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"

	"graduation-bot/config"
)

// Column indices (0-based)
const (
	ColChatID           = 0
	ColUsername         = 1
	ColName             = 2
	ColUniversity       = 3
	ColGradYear         = 4
	ColReferral         = 5
	ColCity             = 6
	ColCompany          = 7
	ColTalk             = 8
	ColCompanions       = 9
	ColSubmitTime       = 10
	ColStatus           = 11
	ColNotified         = 12
	ColPaymentConfirmed = 13
)

// Notifier interface to avoid import cycle
type Notifier interface {
	SendMessage(chatID int64, text string)
	SendApprovedMessage(chatID int64, name string)
}

type Entry struct {
	ChatID     int64
	Username   string
	Name       string
	University string
	GradYear   string
	Referral   string
	City       string
	Company    string
	Talk       string
	Companions string
	SubmitTime string
	Status     string
}

type Client struct {
	svc           *sheets.Service
	spreadsheetID string
	sheetName     string
}

func NewClient(cfg *config.Config) (*Client, error) {
	ctx := context.Background()
	svc, err := sheets.NewService(ctx, option.WithCredentialsFile(cfg.CredentialsFile))
	if err != nil {
		return nil, fmt.Errorf("unable to create sheets service: %w", err)
	}

	return &Client{
		svc:           svc,
		spreadsheetID: cfg.SpreadsheetID,
		sheetName:     cfg.SheetName,
	}, nil
}

// EnsureHeaders writes the header row if the sheet is empty
func (c *Client) EnsureHeaders() error {
	readRange := fmt.Sprintf("%s!A1:N1", c.sheetName)
	resp, err := c.svc.Spreadsheets.Values.Get(c.spreadsheetID, readRange).Do()
	if err != nil {
		return fmt.Errorf("unable to read sheet: %w", err)
	}

	if len(resp.Values) == 0 {
		headers := []interface{}{
			"Chat ID", "Telegram", "Имя Фамилия", "Университет", "Год выпуска",
			"Пригласил", "Город и страна", "Компания", "Доклад", "Спутники",
			"Дата подачи", "Статус", "Уведомлен", "Оплата подтверждена",
		}
		vr := &sheets.ValueRange{
			Values: [][]interface{}{headers},
		}
		_, err = c.svc.Spreadsheets.Values.
			Append(c.spreadsheetID, fmt.Sprintf("%s!A1", c.sheetName), vr).
			ValueInputOption("RAW").Do()

		if err != nil {
			return fmt.Errorf("unable to write headers: %w", err)
		}
		log.Println("Sheet headers created.")
	}
	return nil
}

// IsRegistered checks if a chatID has submitted a registration
func (c *Client) IsRegistered(chatID int64) (bool, error) {
	readRange := fmt.Sprintf("%s!A2:A", c.sheetName)
	resp, err := c.svc.Spreadsheets.Values.Get(c.spreadsheetID, readRange).Do()
	if err != nil {
		return false, fmt.Errorf("unable to read sheet: %w", err)
	}
	chatIDStr := strconv.FormatInt(chatID, 10)
	for _, row := range resp.Values {
		if len(row) > 0 && fmt.Sprintf("%v", row[0]) == chatIDStr {
			return true, nil
		}
	}
	return false, nil
}

// AddEntry appends a new registration entry to the sheet
func (c *Client) AddEntry(entry *Entry) error {
	row := []interface{}{
		strconv.FormatInt(entry.ChatID, 10),
		entry.Username,
		entry.Name,
		entry.University,
		entry.GradYear,
		entry.Referral,
		entry.City,
		entry.Company,
		entry.Talk,
		entry.Companions,
		entry.SubmitTime,
		"pending",
		"no",
	}
	vr := &sheets.ValueRange{
		Values: [][]interface{}{row},
	}
	_, err := c.svc.Spreadsheets.Values.
		Append(c.spreadsheetID, fmt.Sprintf("%s!A:M", c.sheetName), vr).
		ValueInputOption("RAW").Do()
	if err != nil {
		return fmt.Errorf("unable to append entry: %w", err)
	}
	log.Printf("Entry added for chatID=%d name=%s", entry.ChatID, entry.Name)
	return nil
}

// StartPolling checks the sheet every minute for status changes
func (c *Client) StartPolling(notifier Notifier) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	log.Println("📊 Sheet polling started (every 1 minute)...")

	// Run once immediately on start
	c.checkAndNotify(notifier)

	for range ticker.C {
		c.checkAndNotify(notifier)
	}
}

func (c *Client) checkAndNotify(notifier Notifier) {
	readRange := fmt.Sprintf("%s!A2:M", c.sheetName)
	resp, err := c.svc.Spreadsheets.Values.Get(c.spreadsheetID, readRange).Do()
	if err != nil {
		log.Printf("Polling error reading sheet: %v", err)
		return
	}

	for rowIdx, row := range resp.Values {
		if len(row) < 13 {
			continue
		}

		status := fmt.Sprintf("%v", row[ColStatus])
		notified := fmt.Sprintf("%v", row[ColNotified])

		// Only process rows that changed status but weren't notified yet
		if notified == "yes" {
			continue
		}
		if status != "approved" && status != "rejected" {
			continue
		}

		chatIDStr := fmt.Sprintf("%v", row[ColChatID])
		chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
		if err != nil {
			log.Printf("Invalid chatID in row %d: %s", rowIdx+2, chatIDStr)
			continue
		}

		name := fmt.Sprintf("%v", row[ColName])

		switch status {
		case "approved":
			notifier.SendApprovedMessage(chatID, name)
		case "rejected":
			notifier.SendMessage(chatID, fmt.Sprintf(
				"😔 <b>%s</b>, к сожалению, мы не можем подтвердить вашу заявку на участие.\n\n"+
					"Спасибо за интерес к мероприятию! Будем рады видеть вас на других наших событиях 🙏",
				name,
			))
		}
		log.Printf("Notified chatID=%d status=%s", chatID, status)

		// Mark as notified — sheet row is rowIdx+2 (1-indexed, skip header)
		sheetRow := rowIdx + 2
		notifiedRange := fmt.Sprintf("%s!M%d", c.sheetName, sheetRow)
		vr := &sheets.ValueRange{
			Values: [][]interface{}{{"yes"}},
		}
		_, err = c.svc.Spreadsheets.Values.
			Update(c.spreadsheetID, notifiedRange, vr).
			ValueInputOption("RAW").Do()
		if err != nil {
			log.Printf("Failed to mark row %d as notified: %v", sheetRow, err)
		}
	}
}

func (c *Client) ConfirmPayment(username string) error {
	username = strings.TrimPrefix(username, "@")

	readRange := fmt.Sprintf("%s!A2:N", c.sheetName)
	resp, err := c.svc.Spreadsheets.Values.Get(c.spreadsheetID, readRange).Do()
	if err != nil {
		return fmt.Errorf("unable to read sheet: %w", err)
	}

	for rowIdx, row := range resp.Values {
		if len(row) < 2 {
			continue
		}
		rowUsername := strings.TrimPrefix(fmt.Sprintf("%v", row[ColUsername]), "@")
		if strings.EqualFold(rowUsername, username) {
			sheetRow := rowIdx + 2
			paymentRange := fmt.Sprintf("%s!N%d", c.sheetName, sheetRow)
			vr := &sheets.ValueRange{
				Values: [][]interface{}{{"yes"}},
			}
			_, err = c.svc.Spreadsheets.Values.
				Update(c.spreadsheetID, paymentRange, vr).
				ValueInputOption("RAW").Do()
			if err != nil {
				return fmt.Errorf("unable to update payment: %w", err)
			}
			log.Printf("Payment confirmed for username=%s", username)
			return nil
		}
	}
	return fmt.Errorf("user @%s not found in sheet", username)
}

func (c *Client) ConfirmPaymentByChatID(chatID int64) error {
	chatIDStr := strconv.FormatInt(chatID, 10)

	readRange := fmt.Sprintf("%s!A2:N", c.sheetName)
	resp, err := c.svc.Spreadsheets.Values.Get(c.spreadsheetID, readRange).Do()
	if err != nil {
		return fmt.Errorf("unable to read sheet: %w", err)
	}

	for rowIdx, row := range resp.Values {
		if len(row) < 1 {
			continue
		}
		if fmt.Sprintf("%v", row[ColChatID]) == chatIDStr {
			sheetRow := rowIdx + 2
			paymentRange := fmt.Sprintf("%s!N%d", c.sheetName, sheetRow)
			vr := &sheets.ValueRange{
				Values: [][]interface{}{{"yes"}},
			}
			_, err = c.svc.Spreadsheets.Values.
				Update(c.spreadsheetID, paymentRange, vr).
				ValueInputOption("RAW").Do()
			if err != nil {
				return fmt.Errorf("unable to update payment: %w", err)
			}
			log.Printf("Payment confirmed for chatID=%d", chatID)
			return nil
		}
	}
	return fmt.Errorf("chatID %d not found in sheet", chatID)
}
