package bot

import (
	"fmt"
	"log"
	"strconv"
	"strings"
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
			if update.CallbackQuery != nil {
				go b.handleCallback(update.CallbackQuery)
				continue
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
	text := strings.TrimSpace(msg.Text)

	if text == "/start" {
		username := msg.From.UserName
		welcome := "👋 Привет!\nЭто регистрация на <b>Физтехи.Кипр Реюнион'26</b>\n\n📅 <b>16–18 октября</b>\n📍 Лимассол, отель Mediterranean\n💶 <b>170 евро</b> (без проживания)\n\nТри дня лекций, общения и нетворкинга у моря\n\nВ стоимость входит кейтеринг, гала-ужин и кофе-брейки\n\nПодробнее на сайте и в <a href=\"https://t.me/phystechCyprusReunion\">Telegram-группе</a>\n\n<i>Организаторы оставляют за собой право отказать в регистрации или выступлении</i>\n\n⚠️ Пожалуйста, дождитесь подтверждения заявки — участие подтверждается только после ✅ от организаторов.\n\n—\n"
		if username == "" {
			b.sessions.Set(chatID, &Session{Step: StepUsername})
			b.send(chatID, welcome+"У вас не установлен Telegram-ник. Пожалуйста, введите ваш <b>@username</b> вручную (или установите его в настройках Telegram):")
		} else {
			b.sessions.Set(chatID, &Session{Step: StepName, Username: username})
			b.send(chatID, welcome+"Для начала регистрации введите ваше <b>имя и фамилию</b>:")
		}
		return
	}

	if text == "/help" {
		b.send(chatID, "ℹ️ <b>Доступные команды:</b>\n\n"+
			"/start — начать регистрацию\n"+
			"/pay — подтвердить оплату участия")
		return
	}

	if text == "/pay" {
		registered, err := b.sheets.IsRegistered(chatID)
		if err != nil {
			log.Printf("IsRegistered error: %v", err)
		}
		if !registered {
			b.send(chatID, "⚠️ Вы ещё не подали заявку. Введите /start для начала регистрации.")
			return
		}
		username := msg.From.UserName
		if username == "" {
			b.sessions.Set(chatID, &Session{Step: StepPaymentUsername})
			b.send(chatID, "У вас не установлен Telegram-ник. Пожалуйста, введите ваш <b>@username</b>:")
		} else {
			b.sendPaymentConfirmation(chatID, username)
		}
		return
	}

session := b.sessions.Get(chatID)
	if session == nil {
		b.send(chatID, "👋 Привет!\n\n"+
			"/start — подать заявку на участие\n"+
			"/pay — подтвердить оплату после одобрения заявки")
		return
	}

	if len([]rune(text)) > 255 {
		b.send(chatID, "⚠️ Ответ слишком длинный. Пожалуйста, уложитесь в 255 символов.")
		return
	}

	switch session.Step {
	case StepUsername:
		if len(text) < 2 {
			b.send(chatID, "⚠️ Пожалуйста, введите корректный @username.")
			return
		}
		session.Username = text
		session.Step = StepName
		b.sessions.Set(chatID, session)
		b.send(chatID, "Для начала регистрации введите ваше <b>имя и фамилию</b>:")

	case StepPaymentUsername:
		if len(text) < 2 {
			b.send(chatID, "⚠️ Пожалуйста, введите корректный @username.")
			return
		}
		b.sessions.Delete(chatID)
		b.sendPaymentConfirmation(chatID, text)

	case StepName:
		if len(text) < 2 {
			b.send(chatID, "⚠️ Пожалуйста, введите ваше полное имя (минимум 2 символа).")
			return
		}
		session.Name = text
		session.Step = StepUniversity
		b.sessions.Set(chatID, session)
		b.send(chatID, fmt.Sprintf("✅ Отлично, <b>%s</b>!\n\nВведите название вашего <b>университета</b>:", text))

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
		session.Step = StepReferral
		b.sessions.Set(chatID, session)
		b.sendWithSkip(chatID, "👥 Если вы не с МФТИ, напишите Telegram-ник человека, который вас пригласил:")

	case StepReferral:
		if text == "Пропустить" {
			text = "-"
		}
		session.Referral = text
		session.Step = StepCity
		b.sessions.Set(chatID, session)
		b.sendRemoveKeyboard(chatID, "🌍 Укажите <b>город и страну</b>, откуда вы:")

	case StepCity:
		if len(text) < 2 {
			b.send(chatID, "⚠️ Пожалуйста, введите корректный город.")
			return
		}
		session.City = text
		session.Step = StepCompany
		b.sessions.Set(chatID, session)
		b.send(chatID, "💼 Укажите <b>компанию</b>, в которой вы работаете.\n\nЕсли фрилансер или предприниматель — отлично, напишите об этом:")

	case StepCompany:
		if len(text) < 2 {
			b.send(chatID, "⚠️ Пожалуйста, введите корректный ответ.")
			return
		}
		session.Company = text
		session.Step = StepTalk
		b.sessions.Set(chatID, session)
		b.sendWithSkip(chatID, "🎤 Если у вас есть интересная тема для доклада, напишите её здесь.\n\n<i>Организаторы оставляют за собой право не принять некоторые темы докладов.</i>")

	case StepTalk:
		if text == "Пропустить" {
			text = "-"
		}
		session.Talk = text
		session.Step = StepCompanions
		b.sessions.Set(chatID, session)
		b.sendWithSkip(chatID, "🧑‍🤝‍🧑 Если вы будете с кем-то, укажите их Telegram-ники через запятую:")

	case StepCompanions:
		if text == "Пропустить" {
			text = "-"
		}
		session.Companions = text
		session.Step = StepDone
		b.sessions.Set(chatID, session)

		b.sendRemoveKeyboard(chatID, "⏳ Сохраняю вашу заявку...")

		entry := &sheets.Entry{
			ChatID:     chatID,
			Username:   session.Username,
			Name:       session.Name,
			University: session.University,
			GradYear:   session.GradYear,
			Referral:   session.Referral,
			City:       session.City,
			Company:    session.Company,
			Talk:       session.Talk,
			Companions: session.Companions,
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
				"👤 Имя Фамилия: <b>%s</b>\n"+
				"📱 Telegram: <b>%s</b>\n"+
				"🎓 Университет: <b>%s</b>\n"+
				"📅 Год выпуска: <b>%s</b>\n"+
				"👥 Пригласил: <b>%s</b>\n"+
				"🌍 Город и страна: <b>%s</b>\n"+
				"💼 Компания: <b>%s</b>\n"+
				"🎤 Доклад: <b>%s</b>\n"+
				"🧑‍🤝‍🧑 Спутники: <b>%s</b>\n\n"+
				"⏳ Мы получили вашу заявку и уже её рассматриваем.\nБот напишет, когда будут следующие шаги.\n\n"+
				"Если хотите подать новую заявку, введите /start",
			session.Name, session.Username, session.University, session.GradYear,
			session.Referral, session.City, session.Company, session.Talk, session.Companions,
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

func (b *Bot) sendWithSkip(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Пропустить"),
		),
	)
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Error sending message to %d: %v", chatID, err)
	}
}

func (b *Bot) sendRemoveKeyboard(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Error sending message to %d: %v", chatID, err)
	}
}

func (b *Bot) sendPaymentConfirmation(chatID int64, username string) {
	msg := tgbotapi.NewMessage(chatID, "Пожалуйста, подтвердите оплату:")
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Да, я оплатил", "payment_confirm:"+username),
			tgbotapi.NewInlineKeyboardButtonData("❌ Нет", "payment_cancel"),
		),
	)
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Error sending payment confirmation to %d: %v", chatID, err)
	}
}

func (b *Bot) SendApprovedMessage(chatID int64, name string) {
	text := fmt.Sprintf(
		"🎉 <b>Поздравляем, %s!</b>\n\n"+
			"Ваша заявка одобрена ✅\n\n"+
			"Вот ссылка на оплату: [ссылка]\n\n"+
			"После оплаты нажмите кнопку ниже — мы проверим и отправим финальное приглашение.\n\n"+
			"⚠️ Без оплаты мы не сможем окончательно подтвердить участие.\n\n"+
			"До встречи! 🎓🥂",
		name,
	)
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Я оплатил", fmt.Sprintf("payment_confirm_chat:%d", chatID)),
		),
	)
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Error sending approved message to %d: %v", chatID, err)
	}
}

func (b *Bot) handleCallback(cb *tgbotapi.CallbackQuery) {
	b.api.Request(tgbotapi.NewCallback(cb.ID, ""))
	chatID := cb.Message.Chat.ID

	if strings.HasPrefix(cb.Data, "payment_confirm_chat:") {
		targetChatIDStr := strings.TrimPrefix(cb.Data, "payment_confirm_chat:")
		targetChatID, _ := strconv.ParseInt(targetChatIDStr, 10, 64)
		if err := b.sheets.ConfirmPaymentByChatID(targetChatID); err != nil {
			log.Printf("ConfirmPaymentByChatID error: %v", err)
			b.send(chatID, "😔 Не удалось найти вашу заявку. Обратитесь к организаторам.")
		} else {
			b.send(chatID, "✅ Спасибо! Мы проверим оплату и пришлём финальное приглашение.")
		}
	} else if strings.HasPrefix(cb.Data, "payment_confirm:") {
		username := strings.TrimPrefix(cb.Data, "payment_confirm:")
		if err := b.sheets.ConfirmPayment(username); err != nil {
			log.Printf("ConfirmPayment error: %v", err)
			b.send(chatID, "😔 Не удалось найти вашу заявку. Обратитесь к организаторам.")
		} else {
			b.send(chatID, "✅ Спасибо! Мы проверим оплату и пришлём финальное приглашение.")
		}
	} else if cb.Data == "payment_cancel" {
		b.send(chatID, "Окей. Когда оплатите — отправьте /pay.")
	}

	b.api.Request(tgbotapi.NewEditMessageReplyMarkup(chatID, cb.Message.MessageID, tgbotapi.InlineKeyboardMarkup{}))
}
