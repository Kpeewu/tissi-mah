package mocks

import (
	"context"
	"sync"
)

// EmailCall capture un appel à SendEmail pour assertions.
type EmailCall struct {
	To       string
	Subject  string
	BodyText string
	BodyHTML string
}

// SpyEmailSender enregistre tous les appels SendEmail.
// Les appels peuvent être faits depuis des goroutines (envoi asynchrone),
// donc la capture est thread-safe.
type SpyEmailSender struct {
	mu    sync.Mutex
	calls []EmailCall
	// ReturnErr : erreur retournée par SendEmail (nil par défaut).
	ReturnErr error
}

func NewSpyEmailSender() *SpyEmailSender {
	return &SpyEmailSender{}
}

func (s *SpyEmailSender) SendEmail(_ context.Context, to, subject, bodyText, bodyHTML string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, EmailCall{
		To:       to,
		Subject:  subject,
		BodyText: bodyText,
		BodyHTML: bodyHTML,
	})
	return s.ReturnErr
}

// Calls retourne une copie des appels enregistrés.
func (s *SpyEmailSender) Calls() []EmailCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]EmailCall, len(s.calls))
	copy(out, s.calls)
	return out
}

// Count retourne le nombre d'appels SendEmail.
func (s *SpyEmailSender) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

// LastTo retourne l'adresse destinataire du dernier appel, ou "" si aucun.
func (s *SpyEmailSender) LastTo() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.calls) == 0 {
		return ""
	}
	return s.calls[len(s.calls)-1].To
}

// Reset efface l'historique (utile entre sous-tests).
func (s *SpyEmailSender) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = nil
	s.ReturnErr = nil
}
