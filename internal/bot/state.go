package bot

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

type State struct {
	ReminderTopics map[int64]int            `json:"reminder_topics"` // chatID -> topicID
	Reminders      map[string]ReminderState `json:"reminders"`       // key: "chatID_messageID"
	mu             sync.RWMutex             `json:"-"`
}

type ReminderState struct {
	Action    string `json:"action"`
	DateTime  string `json:"date_time"` // в формате RFC3339
	ChatID    int64  `json:"chat_id"`
	MessageID int    `json:"message_id"`
	ThreadID  int64  `json:"thread_id"`
}

const stateFile = "bot_state.json"

func loadState() (*State, error) {
	state := &State{
		ReminderTopics: make(map[int64]int),
		Reminders:      make(map[string]ReminderState),
	}

	data, err := os.ReadFile(stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return state, nil
		}
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	if err := json.Unmarshal(data, state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	return state, nil
}

func (s *State) save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(stateFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

func (s *State) GetTopicID(chatID int64) (int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.ReminderTopics[chatID]
	return id, ok
}

func (s *State) SetTopicID(chatID int64, topicID int) error {
	s.mu.Lock()
	s.ReminderTopics[chatID] = topicID
	s.mu.Unlock()
	return s.save()
}

func (s *State) AddReminder(key string, reminder ReminderState) error {
	s.mu.Lock()
	s.Reminders[key] = reminder
	s.mu.Unlock()
	return s.save()
}

func (s *State) GetReminders() map[string]ReminderState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Создаем копию, чтобы избежать гонки данных
	reminders := make(map[string]ReminderState, len(s.Reminders))
	for k, v := range s.Reminders {
		reminders[k] = v
	}
	return reminders
}

func (s *State) DeleteReminder(key string) error {
	s.mu.Lock()
	delete(s.Reminders, key)
	s.mu.Unlock()
	return s.save()
}
