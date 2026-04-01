package bot

import (
	"fmt"
	"log"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"graduation-bot/config"
	"graduation-bot/sheets"
)

type Bot struct {
	api      *tgbotapi.BotAPI
	sessions *SessionStore
	sheets   *sheets.Client
	stopChan chan struct{}
}

func New(cfg *config.Config, sheetsClient *sheets.Client) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create telegram bot: %w", err)
	}

	log.Printf("Authorized on account @%s", api.Self.UserName)

	return &Bot{
		api:      api,
		sessions: NewSessionStore(),
		sheets:   sheetsClient,
		stopChan: make(chan struct{}),
	}, nil
}

func (b *Bot) Stop() {
	close(b.stopChan)
}

// SendMessage sends a message to a chat by ID (used by the polling goroutine)
func (b *Bot) SendMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Error sending message to %d: %v", chatID, err)
	}
}

// Run starts the update polling loop
func (b *Bot) Run() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	for {
		select {
		case <-b.stopChan:
			return
		case update, ok := <-updates:
			if !ok {
				return
			}
			if update.Message == nil {
				continue
			}
			go b.handleMessage(update.Message)
		}
	}
}

func (b *Bot) handleMessage(msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	text := msg.Text

	if text == "/start" {
		b.sessions.Set(chatID, &Session{Step: StepName})
		b.send(chatID, "🎓 <b>Добро пожаловать на регистрацию выпускников!</b>\n\nЯ помогу вам подать заявку на участие в праздничном мероприятии.\n\nДля начала, пожалуйста, введите ваше <b>полное имя</b>:")
		return
	}

	if text == "/cancel" {
		b.sessions.Delete(chatID)
		b.send(chatID, "❌ Регистрация отменена. Введите /start чтобы начать заново.")
		return
	}

	session := b.sessions.Get(chatID)
	if session == nil {
		b.send(chatID, "👋 Привет! Введите /start для начала регистрации.")
		return
	}

	switch session.Step {
	case StepName:
		if len(text) < 2 {
			b.send(chatID, "⚠️ Пожалуйста, введите ваше полное имя (минимум 2 символа).")
			return
		}
		session.Name = text
		session.Step = StepUniversity
		b.sessions.Set(chatID, session)
		b.send(chatID, fmt.Sprintf("✅ Отлично, <b>%s</b>!\n\nТеперь введите название вашего <b>университета</b>:", text))

	case StepUniversity:
		if len(text) < 2 {
			b.send(chatID, "⚠️ Пожалуйста, введите корректное название университета.")
			return
		}
		session.University = text
		session.Step = StepGradYear
		b.sessions.Set(chatID, session)
		b.send(chatID, "📅 Введите <b>год выпуска</b> (например: 2020):")

	case StepGradYear:
		if len(text) != 4 {
			b.send(chatID, "⚠️ Пожалуйста, введите год в формате ГГГГ (например: 2020).")
			return
		}
		for _, c := range text {
			if c < '0' || c > '9' {
				b.send(chatID, "⚠️ Год должен состоять только из цифр.")
				return
			}
		}
		session.GradYear = text
		session.Step = StepCity
		b.sessions.Set(chatID, session)
		b.send(chatID, "🏙️ Введите ваш <b>город проживания</b>:")

	case StepCity:
		if len(text) < 2 {
			b.send(chatID, "⚠️ Пожалуйста, введите корректный город.")
			return
		}
		session.City = text
		session.Step = StepDone
		b.sessions.Set(chatID, session)

		b.send(chatID, "⏳ Сохраняю вашу заявку...")

		entry := &sheets.Entry{
			ChatID:     chatID,
			Name:       session.Name,
			University: session.University,
			GradYear:   session.GradYear,
			City:       session.City,
			SubmitTime: time.Now().Format("02.01.2006 15:04"),
			Status:     "pending",
		}

		if err := b.sheets.AddEntry(entry); err != nil {
			log.Printf("Error adding entry to sheets: %v", err)
			b.send(chatID, "😔 Произошла ошибка при сохранении заявки. Пожалуйста, попробуйте позже или обратитесь к организаторам.")
			b.sessions.Delete(chatID)
			return
		}

		summary := fmt.Sprintf(
			"✅ <b>Заявка принята!</b>\n\n"+
				"📋 Ваши данные:\n"+
				"👤 Имя: <b>%s</b>\n"+
				"🎓 Университет: <b>%s</b>\n"+
				"📅 Год выпуска: <b>%s</b>\n"+
				"🏙️ Город: <b>%s</b>\n\n"+
				"⏳ Ваша заявка находится на рассмотрении. Мы уведомим вас о решении.\n\n"+
				"Если хотите подать новую заявку, введите /start",
			session.Name, session.University, session.GradYear, session.City,
		)
		b.send(chatID, summary)
		b.sessions.Delete(chatID)

	default:
		b.send(chatID, "Введите /start для начала регистрации.")
	}
}

func (b *Bot) send(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Error sending message to %d: %v", chatID, err)
	}
}
