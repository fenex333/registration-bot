package sheets

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"

	"graduation-bot/config"
)

// Column indices (0-based)
const (
	ColChatID     = 0
	ColName       = 1
	ColUniversity = 2
	ColGradYear   = 3
	ColCity       = 4
	ColSubmitTime = 5
	ColStatus     = 6
	ColNotified   = 7
)

// Notifier interface to avoid import cycle
type Notifier interface {
	SendMessage(chatID int64, text string)
}

type Entry struct {
	ChatID     int64
	Name       string
	University string
	GradYear   string
	City       string
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
	readRange := fmt.Sprintf("%s!A1:H1", c.sheetName)
	resp, err := c.svc.Spreadsheets.Values.Get(c.spreadsheetID, readRange).Do()
	if err != nil {
		return fmt.Errorf("unable to read sheet: %w", err)
	}

	if len(resp.Values) == 0 {
		headers := []interface{}{
			"Chat ID", "Имя", "Университет", "Год выпуска",
			"Город", "Дата подачи", "Статус", "Уведомлен",
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

// AddEntry appends a new registration entry to the sheet
func (c *Client) AddEntry(entry *Entry) error {
	row := []interface{}{
		strconv.FormatInt(entry.ChatID, 10),
		entry.Name,
		entry.University,
		entry.GradYear,
		entry.City,
		entry.SubmitTime,
		"pending",
		"no",
	}
	vr := &sheets.ValueRange{
		Values: [][]interface{}{row},
	}
	_, err := c.svc.Spreadsheets.Values.
		Append(c.spreadsheetID, fmt.Sprintf("%s!A:H", c.sheetName), vr).
		ValueInputOption("RAW").Do()
	if err != nil {
		return fmt.Errorf("unable to append entry: %w", err)
	}
	log.Printf("Entry added for chatID=%d name=%s", entry.ChatID, entry.Name)
	return nil
}

// StartPolling checks the sheet every minute for status changes
func (c *Client) StartPolling(notifier Notifier) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	log.Println("📊 Sheet polling started (every 1 minute)...")

	// Run once immediately on start
	c.checkAndNotify(notifier)

	for range ticker.C {
		c.checkAndNotify(notifier)
	}
}

func (c *Client) checkAndNotify(notifier Notifier) {
	readRange := fmt.Sprintf("%s!A2:H", c.sheetName)
	resp, err := c.svc.Spreadsheets.Values.Get(c.spreadsheetID, readRange).Do()
	if err != nil {
		log.Printf("Polling error reading sheet: %v", err)
		return
	}

	for rowIdx, row := range resp.Values {
		if len(row) < 8 {
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

		var message string
		switch status {
		case "approved":
			message = fmt.Sprintf(
				"🎉 <b>Поздравляем, %s!</b>\n\n"+
					"Ваша заявка на участие в праздничном мероприятии выпускников <b>одобрена</b>! ✅\n\n"+
					"Ждём вас на мероприятии! Подробности и программа будут отправлены ближе к дате события.\n\n"+
					"До встречи! 🎓🥂",
				name,
			)
		case "rejected":
			message = fmt.Sprintf(
				"😔 <b>Уважаемый(ая) %s</b>,\n\n"+
					"К сожалению, мы не можем принять вашу заявку на участие в мероприятии.\n\n"+
					"Возможные причины: мероприятие предназначено для выпускников конкретного университета или набор участников уже завершён.\n\n"+
					"Спасибо за интерес! Мы будем рады видеть вас на других наших событиях. 🙏",
				name,
			)
		}

		notifier.SendMessage(chatID, message)
		log.Printf("Notified chatID=%d status=%s", chatID, status)

		// Mark as notified — sheet row is rowIdx+2 (1-indexed, skip header)
		sheetRow := rowIdx + 2
		notifiedRange := fmt.Sprintf("%s!H%d", c.sheetName, sheetRow)
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
