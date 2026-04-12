package bot

import (
	"sync"
	"time"
)

type Step int

const (
	StepIdle Step = iota
	StepUsername
	StepPaymentUsername
	StepName
	StepUniversity
	StepGradYear
	StepReferral
	StepCity
	StepCompany
	StepTalk
	StepCompanions
	StepDone
)

const sessionTTL = 24 * time.Hour

type Session struct {
	Step         Step
	Username     string
	Name         string
	University   string
	GradYear     string
	Referral     string
	City         string
	Company      string
	Talk         string
	Companions   string
	LastActivity time.Time
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
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[chatID]
	if !ok {
		return nil
	}
	if time.Since(session.LastActivity) > sessionTTL {
		delete(s.sessions, chatID)
		return nil
	}

	copy := *session
	return &copy
}

func (s *SessionStore) Set(chatID int64, session *Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session.LastActivity = time.Now()
	s.sessions[chatID] = session
}

func (s *SessionStore) Delete(chatID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, chatID)
}
