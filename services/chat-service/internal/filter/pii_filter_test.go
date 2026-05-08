package filter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPIIFilter(t *testing.T) {
	f := NewPIIFilter()

	cases := []struct {
		name           string
		input          string
		wantRedacted   bool
		wantContains   string
		wantNotContain string
	}{
		{
			name:         "Numéro Togo format local",
			input:        "Appelle moi au 90 12 34 56 stp",
			wantRedacted: true,
			wantNotContain: "90 12 34 56",
		},
		{
			name:         "Numéro Togo avec +228",
			input:        "Mon numéro est +22890123456",
			wantRedacted: true,
			wantNotContain: "+22890123456",
		},
		{
			name:         "Email simple",
			input:        "Écris-moi à maxime@gmail.com pour les détails",
			wantRedacted: true,
			wantNotContain: "maxime@gmail.com",
		},
		{
			name:         "URL HTTPS externe",
			input:        "Retrouve moi sur https://wa.me/22890123456",
			wantRedacted: true,
			wantNotContain: "https://wa.me",
		},
		{
			name:         "URL HTTP non-sécurisée bloquée",
			input:        "Va voir http://exemple.com/promo",
			wantRedacted: true,
			wantNotContain: "http://exemple.com",
		},
		{
			name:         "URL FTP bloquée",
			input:        "Récupère sur ftp://files.example.org/data",
			wantRedacted: true,
			wantNotContain: "ftp://",
		},
		{
			name:         "Google Maps conservé (https)",
			input:        "Retrouve moi ici https://maps.google.com/maps?q=6.13,1.22",
			wantRedacted: false,
			wantContains: "maps.google.com",
		},
		{
			name:         "Google Maps conservé (sans scheme)",
			input:        "RDV: maps.app.goo.gl/abc123xyz",
			wantRedacted: false,
			wantContains: "maps.app.goo.gl",
		},
		{
			name:         "Raccourcisseur bit.ly bloqué",
			input:        "Mon profil : bit.ly/maxime-tg",
			wantRedacted: true,
			wantNotContain: "bit.ly/",
		},
		{
			name:         "Raccourcisseur tinyurl bloqué",
			input:        "Lien tinyurl.com/abc123",
			wantRedacted: true,
			wantNotContain: "tinyurl.com/abc123",
		},
		{
			name:         "Domaine brut .com bloqué",
			input:        "Va sur monsite.com/contact",
			wantRedacted: true,
			wantNotContain: "monsite.com",
		},
		{
			name:         "Domaine .tg bloqué",
			input:        "Site officiel : example.tg",
			wantRedacted: true,
			wantNotContain: "example.tg",
		},
		{
			name:         "Scheme tel: bloqué",
			input:        "Appelle tel:+22890123456",
			wantRedacted: true,
			wantNotContain: "tel:",
		},
		{
			name:         "Scheme whatsapp:// bloqué",
			input:        "Ouvre whatsapp://send?phone=22890123456",
			wantRedacted: true,
			wantNotContain: "whatsapp://",
		},
		{
			name:         "Scheme tg:// bloqué",
			input:        "Ouvre tg://resolve?domain=monchan",
			wantRedacted: true,
			wantNotContain: "tg://",
		},
		{
			name:         "Instagram URL bloquée",
			input:        "Suis-moi sur instagram.com/maxime",
			wantRedacted: true,
			wantNotContain: "instagram.com/maxime",
		},
		{
			name:         "www. URL bloquée",
			input:        "Visite www.example.org/page",
			wantRedacted: true,
			wantNotContain: "www.example.org",
		},
		{
			name:         "Handle @username",
			input:        "Ajoute moi sur @maxdrive_tg",
			wantRedacted: true,
			wantNotContain: "@maxdrive_tg",
		},
		{
			name:         "Telegram link",
			input:        "Je suis sur t.me/maxime_togo",
			wantRedacted: true,
			wantNotContain: "t.me/maxime_togo",
		},
		{
			name:         "Texte propre sans PII",
			input:        "Je serai à l'arrêt de bus à 14h00",
			wantRedacted: false,
			wantContains: "14h00",
		},
		{
			name:         "Plusieurs PII dans un message",
			input:        "Mon mail: test@test.com et tel: +22891234567",
			wantRedacted: true,
			wantNotContain: "test@test.com",
		},
		{
			name:         "Message vide",
			input:        "",
			wantRedacted: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, redacted := f.Filter(c.input)
			assert.Equal(t, c.wantRedacted, redacted, "redaction flag mismatch")
			if c.wantContains != "" {
				assert.Contains(t, got, c.wantContains)
			}
			if c.wantNotContain != "" {
				assert.NotContains(t, got, c.wantNotContain)
			}
		})
	}
}
