package bot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"alert_bot/internal/config"
	"alert_bot/internal/reminder"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/robfig/cron/v3"
)

const (
	reminderTopicName = "📅 Напоминания"
	telegramAPIURL    = "https://api.telegram.org/bot%s/%s"
)

type Bot struct {
	api      *tgbotapi.BotAPI
	config   *config.Config
	reminder *reminder.Service
	cron     *cron.Cron
	state    *State
}

// Структуры для работы с Telegram API
type ForumTopic struct {
	MessageThreadID int    `json:"message_thread_id"`
	Name            string `json:"name"`
}

type CreateTopicRequest struct {
	ChatID int64  `json:"chat_id"`
	Name   string `json:"name"`
}

type CreateTopicResponse struct {
	Ok     bool       `json:"ok"`
	Result ForumTopic `json:"result"`
}

type SendMessageRequest struct {
	ChatID          int64  `json:"chat_id"`
	Text            string `json:"text"`
	MessageThreadID int    `json:"message_thread_id"`
}

func (b *Bot) createOrGetReminderTopic(chatID int64) (int, error) {
	// Проверяем, есть ли уже сохраненный ID топика для этого чата
	if topicID, ok := b.state.GetTopicID(chatID); ok {
		return topicID, nil
	}

	// Пытаемся найти существующий топик с нужным именем
	url := fmt.Sprintf(telegramAPIURL, b.config.TelegramToken, "getForumTopicsByChat")
	reqBody := struct {
		ChatID int64 `json:"chat_id"`
	}{
		ChatID: chatID,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err == nil {
		var result struct {
			Ok     bool `json:"ok"`
			Result []struct {
				MessageThreadID int    `json:"message_thread_id"`
				Name            string `json:"name"`
			} `json:"result"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err == nil && result.Ok {
			for _, topic := range result.Result {
				if topic.Name == reminderTopicName {
					// Нашли существующий топик
					if err := b.state.SetTopicID(chatID, topic.MessageThreadID); err != nil {
						log.Printf("Failed to save topic ID: %v", err)
					}
					return topic.MessageThreadID, nil
				}
			}
		}
		resp.Body.Close()
	}

	// Если не нашли существующий топик, создаем новый
	url = fmt.Sprintf(telegramAPIURL, b.config.TelegramToken, "createForumTopic")
	createReqBody := CreateTopicRequest{
		ChatID: chatID,
		Name:   reminderTopicName,
	}

	jsonData, err = json.Marshal(createReqBody)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err = http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return 0, fmt.Errorf("failed to create topic: %w", err)
	}
	defer resp.Body.Close()

	var result CreateTopicResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	if !result.Ok {
		return 0, fmt.Errorf("failed to create topic")
	}

	// Сохраняем ID нового топика
	if err := b.state.SetTopicID(chatID, result.Result.MessageThreadID); err != nil {
		log.Printf("Failed to save topic ID: %v", err)
	}
	return result.Result.MessageThreadID, nil
}

func (b *Bot) sendToReminderTopic(chatID int64, text string) error {
	topicID, err := b.createOrGetReminderTopic(chatID)
	if err != nil {
		return err
	}

	url := fmt.Sprintf(telegramAPIURL, b.config.TelegramToken, "sendMessage")
	reqBody := SendMessageRequest{
		ChatID:          chatID,
		Text:            text,
		MessageThreadID: topicID,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	return nil
}

func (b *Bot) Start() error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	// Запускаем проверку напоминаний каждую минуту
	b.cron.AddFunc("* * * * *", func() {
		now := time.Now()
		log.Printf("=== Проверка напоминаний [%s] ===", now.Format("15:04:05"))
		log.Printf("Время напоминания установлено на %d минут", int(b.config.ReminderTime.Minutes()))

		reminders := b.state.GetReminders()
		log.Printf("Всего активных напоминаний: %d", len(reminders))

		for key, r := range reminders {
			dt, err := time.Parse(time.RFC3339, r.DateTime)
			if err != nil {
				log.Printf("Failed to parse reminder date: %v", err)
				continue
			}

			timeUntilEvent := dt.Sub(now)
			reminderTime := b.config.ReminderTime

			if timeUntilEvent > 0 && timeUntilEvent <= reminderTime {
				text := fmt.Sprintf("🔔 Напоминание: через %d минут - %s",
					int(reminderTime.Minutes()), r.Action)

				if err := b.sendToReminderTopic(r.ChatID, text); err != nil {
					log.Printf("❌ Ошибка отправки напоминания: %v", err)
				} else {
					log.Printf("✅ Отправлено напоминание для '%s' на %s", r.Action, dt.Format("15:04 02.01.2006"))
					// Удаляем отправленное напоминание
					if err := b.state.DeleteReminder(key); err != nil {
						log.Printf("Failed to delete reminder: %v", err)
					}
				}
			} else if timeUntilEvent <= 0 {
				// Удаляем просроченные напоминания
				if err := b.state.DeleteReminder(key); err != nil {
					log.Printf("Failed to delete expired reminder: %v", err)
				}
			}
		}

		log.Println("=== Проверка завершена ===")
	})
	b.cron.Start()

	// Обрабатываем входящие сообщения
	for update := range updates {
		if update.Message == nil {
			continue
		}

		// Логируем информацию о сообщении для отладки
		log.Printf("Получено сообщение: [%s] в чате %d (тип: %s)",
			update.Message.Text,
			update.Message.Chat.ID,
			update.Message.Chat.Type)

		// Проверяем, является ли сообщение командой
		if update.Message.IsCommand() {
			switch update.Message.Command() {
			case "start":
				text := "👋 Привет! Я бот для напоминаний. Отправь мне сообщение в формате:\n\n" +
					"Действие\nДД.ММ.ГГГГ в ЧЧ:ММ\n\n" +
					"Например:\nСрать\n13.03.2025 в 21:04"
				b.sendToReminderTopic(update.Message.Chat.ID, text)
				continue
			case "help":
				text := "📝 Поддерживаемые форматы даты:\n\n" +
					"1. ДД.ММ.ГГГГ в ЧЧ:ММ\n" +
					"2. ДД.ММ ЧЧ:ММ\n" +
					"3. Завтра в Х\n" +
					"4. в ЧЧ:ММ (сегодня/завтра)"
				b.sendToReminderTopic(update.Message.Chat.ID, text)
				continue
			}
		}

		action, dateTimeStr, ok := b.reminder.ParseMessage(update.Message.Text)
		if !ok {
			continue
		}

		dt, ok := b.reminder.AddReminder(action, dateTimeStr, update.Message.Chat.ID, update.Message.MessageID, 0)
		if !ok {
			continue
		}

		// Сохраняем напоминание в состояние
		key := fmt.Sprintf("%d_%d", update.Message.Chat.ID, update.Message.MessageID)
		reminder := ReminderState{
			Action:    action,
			DateTime:  dt.Format(time.RFC3339),
			ChatID:    update.Message.Chat.ID,
			MessageID: update.Message.MessageID,
		}
		if err := b.state.AddReminder(key, reminder); err != nil {
			log.Printf("Failed to save reminder: %v", err)
		}

		reply := fmt.Sprintf("✅ Запланировано: %s\nДата и время: %s",
			action,
			dt.Format("15:04 02.01.2006"))

		b.sendToReminderTopic(update.Message.Chat.ID, reply)
	}

	return nil
}

func (b *Bot) Stop() {
	b.cron.Stop()
}

func New(cfg *config.Config) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	api.Debug = cfg.Debug
	log.Printf("Authorized on account %s", api.Self.UserName)

	state, err := loadState()
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	return &Bot{
		api:      api,
		config:   cfg,
		reminder: reminder.New(cfg),
		cron:     cron.New(),
		state:    state,
	}, nil
}
