package provider

import "context"

// EmailProvider est l'interface abstraite pour l'envoi d'emails.
// Implémentée par SendGridProvider (dev/staging) et SESProvider (prod).
type EmailProvider interface {
	// Send envoie un email et retourne l'ID du message chez le fournisseur.
	Send(ctx context.Context, to, subject, bodyText, bodyHTML string) (providerMessageID string, err error)

	// Name retourne le nom du fournisseur (sendgrid, aws_ses).
	Name() string
}
