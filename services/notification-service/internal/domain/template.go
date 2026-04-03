package domain

import (
	"strings"
	"time"
)

// Template représente un template de notification.
type Template struct {
	TemplateID   string
	EventType    string
	Channel      string // "push", "email"
	LanguageCode string
	Title        string
	Subject      string
	Body         string
	BodyHTML     string
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ResolvedTemplate contient les textes après substitution des variables.
type ResolvedTemplate struct {
	Title    string
	Subject  string
	Body     string
	BodyHTML string
}

// ResolveTemplate substitue les variables {{key}} par les valeurs du payload.
func ResolveTemplate(t *Template, payload map[string]string) ResolvedTemplate {
	return ResolvedTemplate{
		Title:    replaceVars(t.Title, payload),
		Subject:  replaceVars(t.Subject, payload),
		Body:     replaceVars(t.Body, payload),
		BodyHTML: replaceVars(t.BodyHTML, payload),
	}
}

func replaceVars(text string, vars map[string]string) string {
	for key, value := range vars {
		text = strings.ReplaceAll(text, "{{"+key+"}}", value)
	}
	return text
}
