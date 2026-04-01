# 🎓 Graduation Event Registration Bot

Telegram-бот для регистрации выпускников на праздничное мероприятие с интеграцией Google Sheets.

## 📋 Возможности

- Пошаговый сбор данных участника (имя, университет, год выпуска, город)
- Запись заявок в Google Sheets со статусом `pending`
- Поллинг таблицы каждую минуту
- Уведомление участника при изменении статуса на `approved` или `rejected`
- Команды `/start` и `/cancel`

## 🗂 Структура проекта

```
graduation-bot/
├── main.go              # Точка входа
├── config/
│   └── config.go        # Загрузка конфигурации
├── bot/
│   ├── bot.go           # Логика бота и обработка сообщений
│   └── session.go       # Управление состоянием диалога
├── sheets/
│   └── client.go        # Google Sheets клиент + поллинг
├── .env.example         # Пример переменных окружения
├── Dockerfile
├── docker-compose.yml
└── go.mod
```

## 🚀 Быстрый старт

### 1. Создайте Telegram-бота

1. Откройте [@BotFather](https://t.me/BotFather) в Telegram
2. Отправьте `/newbot` и следуйте инструкциям
3. Скопируйте полученный **токен**

### 2. Настройте Google Sheets API

1. Откройте [Google Cloud Console](https://console.cloud.google.com/)
2. Создайте новый проект (или выберите существующий)
3. Включите **Google Sheets API**:
   - Перейдите в *APIs & Services → Library*
   - Найдите "Google Sheets API" → Enable
4. Создайте Service Account:
   - *APIs & Services → Credentials → Create Credentials → Service Account*
   - Дайте любое имя, нажмите *Done*
5. Создайте JSON-ключ:
   - Откройте созданный сервисный аккаунт
   - Вкладка *Keys → Add Key → Create new key → JSON*
   - Скачайте файл и переименуйте в `credentials.json`
   - Скопируйте в папку проекта
6. Создайте Google Spreadsheet:
   - Откройте [Google Sheets](https://sheets.google.com) → новая таблица
   - Скопируйте ID из URL: `https://docs.google.com/spreadsheets/d/**SPREADSHEET_ID**/edit`
7. Дайте доступ сервисному аккаунту:
   - В таблице: *Поделиться → вставьте email сервисного аккаунта* (из credentials.json, поле `client_email`)
   - Роль: **Редактор**

### 3. Настройте переменные окружения

```bash
cp .env.example .env
```

Отредактируйте `.env`:

```env
TELEGRAM_TOKEN=1234567890:ABCdefGHIjklMNOpqrSTUvwxYZ
SPREADSHEET_ID=1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgVE2upms
SHEET_NAME=Заявки
GOOGLE_CREDENTIALS_FILE=credentials.json
```

### 4. Запустите бота

#### Локально (Go 1.21+)

```bash
go mod tidy
go run .
```

#### Docker

```bash
docker-compose up -d
```

## 📊 Структура Google Sheets

| Chat ID | Имя | Университет | Год выпуска | Город | Дата подачи | Статус | Уведомлен |
|---------|-----|-------------|-------------|-------|-------------|--------|-----------|
| 123456  | Иван Петров | МГУ | 2020 | Москва | 01.04.2026 14:30 | pending | no |

### Управление заявками (для организатора)

Измените значение в столбце **Статус**:

| Значение | Действие |
|----------|----------|
| `pending` | Заявка ожидает рассмотрения (начальное значение) |
| `approved` | ✅ Бот отправит участнику подтверждение |
| `rejected` | ❌ Бот отправит вежливый отказ |

> Бот проверяет таблицу каждую **1 минуту**. После уведомления столбец "Уведомлен" автоматически принимает значение `yes` — повторного сообщения не будет.

## 💬 Сценарий диалога

```
Пользователь: /start
Бот: 🎓 Добро пожаловать на регистрацию выпускников!
     Введите ваше полное имя:

Пользователь: Иван Петров
Бот: ✅ Отлично, Иван Петров! Введите название вашего университета:

Пользователь: МГУ
Бот: 📅 Введите год выпуска (например: 2020):

Пользователь: 2019
Бот: 🏙️ Введите ваш город проживания:

Пользователь: Москва
Бот: ✅ Заявка принята! Ваша заявка находится на рассмотрении...

--- Организатор меняет статус на "approved" ---

Бот: 🎉 Поздравляем, Иван Петров! Ваша заявка одобрена!...
```

## 🛠 Команды бота

| Команда | Описание |
|---------|----------|
| `/start` | Начать регистрацию |
| `/cancel` | Отменить текущую регистрацию |

## ⚙️ Технологии

- **Go 1.21**
- [go-telegram-bot-api](https://github.com/go-telegram-bot-api/telegram-bot-api) — Telegram Bot API
- [google.golang.org/api](https://pkg.go.dev/google.golang.org/api) — Google Sheets API
- [godotenv](https://github.com/joho/godotenv) — загрузка `.env`
