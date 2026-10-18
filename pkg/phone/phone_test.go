package phone_test

import (
	"testing"

	"github.com/Kpeewu/tissi-mah/pkg/phone"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantOK  bool
		wantCC  string
		wantE164 string
		wantLocal string
	}{
		// Togo — formats E.164
		{"togo +228", "+22890123456", true, "tg", "+22890123456", "90123456"},
		{"togo +228 espaces", "+228 90 12 34 56", true, "tg", "+22890123456", "90123456"},
		{"togo 00228", "0022890123456", true, "tg", "+22890123456", "90123456"},
		// Togo — fallback 8 chiffres nus (rétrocompat)
		{"togo 8 chiffres", "90123456", true, "tg", "+22890123456", "90123456"},
		{"togo 8 chiffres moov", "70123456", true, "tg", "+22870123456", "70123456"},
		// Bénin — formats E.164
		{"benin +229", "+22990123456", true, "bj", "+22990123456", "90123456"},
		{"benin 00229", "0022990123456", true, "bj", "+22990123456", "90123456"},
		{"benin +229 tirets", "+229-90-12-34-56", true, "bj", "+22990123456", "90123456"},
		// Invalides
		{"vide", "", false, "", "", ""},
		{"trop court", "+228901234", false, "", "", ""},
		{"trop long", "+2289012345678", false, "", "", ""},
		{"caractère invalide", "+228901a3456", false, "", "", ""},
		{"pays non supporté", "+33612345678", false, "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := phone.Normalize(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("Normalize(%q) ok=%v, want %v", tt.input, ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if got.CountryCode != tt.wantCC {
				t.Errorf("CountryCode=%q, want %q", got.CountryCode, tt.wantCC)
			}
			if got.E164 != tt.wantE164 {
				t.Errorf("E164=%q, want %q", got.E164, tt.wantE164)
			}
			if got.LocalNumber != tt.wantLocal {
				t.Errorf("LocalNumber=%q, want %q", got.LocalNumber, tt.wantLocal)
			}
		})
	}
}

func TestIsValidOperator(t *testing.T) {
	tests := []struct {
		cc       string
		operator string
		want     bool
	}{
		// Togo
		{"tg", "togocel", true},
		{"tg", "moov", false},
		{"tg", "mtn_open", false},
		// Bénin
		{"bj", "mtn_open", true},
		{"bj", "moov", true},
		{"bj", "sbin", true},
		{"bj", "togocel", false},
		// Pays inconnu
		{"gh", "mtn", false},
		{"", "togocel", false},
	}

	for _, tt := range tests {
		t.Run(tt.cc+"/"+tt.operator, func(t *testing.T) {
			got := phone.IsValidOperator(tt.cc, tt.operator)
			if got != tt.want {
				t.Errorf("IsValidOperator(%q, %q)=%v, want %v", tt.cc, tt.operator, got, tt.want)
			}
		})
	}
}
