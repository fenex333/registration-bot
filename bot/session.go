package bot

import "sync"

type Step int

const (
	StepIdle Step = iota
	StepName
	StepUniversity
	StepGradYear
	StepCity
	StepDone
)

type Session struct {
	Step       Step
	Name       string
	University string
	GradYear   string
	City       string
}

type SessionStore struct {
	mu       sync.RWMutex
	sessions map[int64]*Session
}

func NewSessionStore() *SessionStore {
	return &SessionStore{
		sessions: make(map[int64]*Session),
	}
}

func (s *SessionStore) Get(chatID int64) *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.sessions[chatID] == nil {
		return nil
	}

	var session = *s.sessions[chatID]
	return &session
}

func (s *SessionStore) Set(chatID int64, session *Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[chatID] = session
}

func (s *SessionStore) Delete(chatID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, chatID)
}
