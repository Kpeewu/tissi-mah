package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestComputeMinutesFromDepartureTime(t *testing.T) {
	s := &tripServiceImpl{}

	mustParse := func(t *testing.T, value string) time.Time {
		t.Helper()
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		return parsed
	}

	t.Run("même jour, 2h plus tard", func(t *testing.T) {
		departure := mustParse(t, "2026-04-20T14:30:00Z")
		got := s.computeMinutesFromDepartureTime("2026-04-20T16:30:00Z", departure)
		assert.Equal(t, 120, got)
	})

	t.Run("overnight wrap (arrivée après minuit)", func(t *testing.T) {
		departure := mustParse(t, "2026-04-20T23:00:00Z")
		got := s.computeMinutesFromDepartureTime("2026-04-21T01:00:00Z", departure)
		assert.Equal(t, 120, got)
	})

	t.Run("overnight limite (arrivée à minuit pile)", func(t *testing.T) {
		departure := mustParse(t, "2026-04-20T23:00:00Z")
		got := s.computeMinutesFromDepartureTime("2026-04-21T00:00:00Z", departure)
		assert.Equal(t, 60, got)
	})

	t.Run("même heure exacte → 0", func(t *testing.T) {
		departure := mustParse(t, "2026-04-20T14:30:00Z")
		got := s.computeMinutesFromDepartureTime("2026-04-20T14:30:00Z", departure)
		assert.Equal(t, 0, got)
	})

	t.Run("scheduledStr vide → 0", func(t *testing.T) {
		departure := mustParse(t, "2026-04-20T14:30:00Z")
		got := s.computeMinutesFromDepartureTime("", departure)
		assert.Equal(t, 0, got)
	})

	t.Run("format invalide → 0", func(t *testing.T) {
		departure := mustParse(t, "2026-04-20T14:30:00Z")
		got := s.computeMinutesFromDepartureTime("not-a-date", departure)
		assert.Equal(t, 0, got)
	})
}
