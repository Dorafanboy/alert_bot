package reminder

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"alert_bot/internal/config"
)

type Reminder struct {
	Action    string
	DateTime  time.Time
	ChatID    int64
	MessageID int
	ThreadID  int64
}

type Service struct {
	reminders     map[string]Reminder
	dateRegex     *regexp.Regexp
	tomorrowRegex *regexp.Regexp
	config        *config.Config
}

func New(cfg *config.Config) *Service {
	return &Service{
		reminders:     make(map[string]Reminder),
		dateRegex:     regexp.MustCompile(`(\d{2}\.\d{2})\s+(\d{2}:\d{2})`),
		tomorrowRegex: regexp.MustCompile(`(?i)завтра\s+в\s+(\d{1,2})`),
		config:        cfg,
	}
}

func (s *Service) parseDateTime(text string) (time.Time, bool) {
	loc, err := time.LoadLocation(s.config.Timezone)
	if err != nil {
		loc = time.Local
	}

	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	// Проверяем формат "DD.MM.YYYY в HH:MM" или "DD.MM.YYYY в HH.MM"
	if matches := regexp.MustCompile(`(\d{2}\.\d{2}\.\d{4})\s+в\s+(\d{2})[:\.](\d{2})`).FindStringSubmatch(text); len(matches) > 0 {
		dateStr := matches[1]
		hour, _ := strconv.Atoi(matches[2])
		minute, _ := strconv.Atoi(matches[3])

		dt, err := time.ParseInLocation("02.01.2006", dateStr, loc)
		if err == nil {
			dt = time.Date(dt.Year(), dt.Month(), dt.Day(), hour, minute, 0, 0, loc)
			return dt, true
		}
	}

	// Проверяем формат "DD.MM HH:MM"
	if matches := s.dateRegex.FindStringSubmatch(text); len(matches) > 0 {
		dateStr := matches[1]
		timeStr := matches[2]
		fullDateStr := fmt.Sprintf("%s.%d %s", dateStr, now.Year(), timeStr)
		dt, err := time.ParseInLocation("02.01.2006 15:04", fullDateStr, loc)
		if err == nil {
			if dt.Before(now) {
				dt = dt.AddDate(1, 0, 0)
			}
			return dt, true
		}
	}

	// Проверяем формат "Завтра в X"
	if matches := s.tomorrowRegex.FindStringSubmatch(text); len(matches) > 0 {
		hour, _ := strconv.Atoi(matches[1])
		tomorrow := today.Add(24 * time.Hour)
		dt := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), hour, 0, 0, 0, loc)
		return dt, true
	}

	// Проверяем формат "в HH:MM" или "в HH.MM" (сегодня)
	if matches := regexp.MustCompile(`(?i)в\s+(\d{1,2})[:\.](\d{2})`).FindStringSubmatch(text); len(matches) > 0 {
		hour, _ := strconv.Atoi(matches[1])
		minute, _ := strconv.Atoi(matches[2])
		dt := time.Date(today.Year(), today.Month(), today.Day(), hour, minute, 0, 0, loc)
		if dt.Before(now) {
			dt = dt.Add(24 * time.Hour)
		}
		return dt, true
	}

	// Проверяем формат "в X" (сегодня)
	if matches := regexp.MustCompile(`(?i)в\s+(\d{1,2})`).FindStringSubmatch(text); len(matches) > 0 {
		hour, _ := strconv.Atoi(matches[1])
		dt := time.Date(today.Year(), today.Month(), today.Day(), hour, 0, 0, 0, loc)
		if dt.Before(now) {
			dt = dt.Add(24 * time.Hour)
		}
		return dt, true
	}

	// Проверяем простой формат "HH:MM" или "HH.MM" (сегодня)
	if matches := regexp.MustCompile(`^(\d{1,2})[:\.](\d{2})$`).FindStringSubmatch(text); len(matches) > 0 {
		hour, _ := strconv.Atoi(matches[1])
		minute, _ := strconv.Atoi(matches[2])
		dt := time.Date(today.Year(), today.Month(), today.Day(), hour, minute, 0, 0, loc)
		if dt.Before(now) {
			dt = dt.Add(24 * time.Hour)
		}
		return dt, true
	}

	return time.Time{}, false
}

func (s *Service) AddReminder(action string, dateTimeStr string, chatID int64, messageID int, threadID int64) (time.Time, bool) {
	dt, ok := s.parseDateTime(dateTimeStr)
	if !ok {
		return time.Time{}, false
	}

	id := fmt.Sprintf("%d_%d", chatID, messageID)
	s.reminders[id] = Reminder{
		Action:    action,
		DateTime:  dt,
		ChatID:    chatID,
		MessageID: messageID,
		ThreadID:  threadID,
	}

	return dt, true
}

func (s *Service) GetUpcomingReminders() []Reminder {
	now := time.Now()
	var upcoming []Reminder

	for _, reminder := range s.reminders {
		// Вычисляем время до события
		timeUntilEvent := reminder.DateTime.Sub(now)
		reminderTime := s.config.ReminderTime

		// Проверяем, находимся ли мы в минутном интервале для отправки уведомления
		if timeUntilEvent > 0 && // Событие еще не наступило
			timeUntilEvent <= reminderTime && // До события осталось меньше или равно времени напоминания
			timeUntilEvent > (reminderTime-time.Minute) { // Но больше чем (время напоминания - 1 минута)
			upcoming = append(upcoming, reminder)
		}
	}

	return upcoming
}

func (s *Service) CleanupExpiredReminders() {
	now := time.Now()
	for id, reminder := range s.reminders {
		if now.After(reminder.DateTime) {
			log.Printf("Удаляем просроченное напоминание: %s запланированное на %s",
				reminder.Action,
				reminder.DateTime.Format("15:04 02.01.2006"))
			delete(s.reminders, id)
		}
	}
}

func (s *Service) ParseMessage(text string) (action string, dateTimeStr string, ok bool) {
	// Удаляем лишние пробелы и переносы строк
	text = strings.TrimSpace(text)

	// Пытаемся найти формат "Действие DD.MM.YYYY в HH:MM" или "Действие DD.MM.YYYY в HH.MM"
	if matches := regexp.MustCompile(`^(.+?)\s+(\d{2}\.\d{2}\.\d{4})\s+в\s+(\d{2})[:\.](\d{2})`).FindStringSubmatch(text); len(matches) > 0 {
		action = strings.TrimSpace(matches[1])
		dateTimeStr = fmt.Sprintf("%s в %s:%s", matches[2], matches[3], matches[4])
		return action, dateTimeStr, true
	}

	// Разбиваем на строки и фильтруем пустые
	lines := strings.Split(text, "\n")
	var nonEmptyLines []string
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			nonEmptyLines = append(nonEmptyLines, trimmed)
		}
	}

	// Если есть только одна строка, пробуем найти время в ней
	if len(nonEmptyLines) == 1 {
		line := nonEmptyLines[0]
		// Ищем время в формате "в HH:MM" или "в HH.MM"
		if timeMatch := regexp.MustCompile(`в\s+(\d{1,2})[:\.](\d{2})`).FindStringSubmatch(line); len(timeMatch) > 0 {
			action = strings.TrimSpace(strings.Replace(line, timeMatch[0], "", 1))
			dateTimeStr = fmt.Sprintf("%s:%s", timeMatch[1], timeMatch[2])
			return action, dateTimeStr, true
		}
		// Ищем время в формате "в X"
		if timeMatch := regexp.MustCompile(`в\s+(\d{1,2})`).FindStringSubmatch(line); len(timeMatch) > 0 {
			action = strings.TrimSpace(strings.Replace(line, timeMatch[0], "", 1))
			dateTimeStr = "Завтра в " + timeMatch[1]
			return action, dateTimeStr, true
		}
		return "", "", false
	}

	// Если есть две или более строк, используем последние две
	if len(nonEmptyLines) >= 2 {
		action = nonEmptyLines[len(nonEmptyLines)-2]
		dateTimeStr = nonEmptyLines[len(nonEmptyLines)-1]
		return action, dateTimeStr, true
	}

	return "", "", false
}

func (s *Service) GetReminders() map[string]Reminder {
	return s.reminders
}

func (s *Service) DeleteReminder(id string) {
	delete(s.reminders, id)
}
