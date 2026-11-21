package service

import (
	"testing"
	"time"
)

func TestPaymentWindow_IsOpen(t *testing.T) {
	lome, err := time.LoadLocation("Africa/Lome")
	if err != nil {
		t.Fatalf("load Africa/Lome: %v", err)
	}

	mk := func(loc *time.Location, h, m, s int) time.Time {
		return time.Date(2026, 5, 1, h, m, s, 0, loc)
	}

	cases := []struct {
		name    string
		w       PaymentWindow
		t       time.Time
		wantOpen bool
	}{
		{
			name:     "Disabled retourne toujours true (hors fenêtre nominale)",
			w:        PaymentWindow{Enabled: false, Location: lome, StartHour: 0, EndHour: 3},
			t:        mk(lome, 12, 0, 0),
			wantOpen: true,
		},
		{
			name:     "Disabled sans Location",
			w:        PaymentWindow{Enabled: false},
			t:        mk(time.UTC, 12, 0, 0),
			wantOpen: true,
		},
		{
			name:     "00h00 pile, fenêtre [0,3)",
			w:        PaymentWindow{Enabled: true, Location: lome, StartHour: 0, EndHour: 3},
			t:        mk(lome, 0, 0, 0),
			wantOpen: true,
		},
		{
			name:     "02h59:59, fenêtre [0,3)",
			w:        PaymentWindow{Enabled: true, Location: lome, StartHour: 0, EndHour: 3},
			t:        mk(lome, 2, 59, 59),
			wantOpen: true,
		},
		{
			name:     "03h00:00 pile (exclusif), fenêtre [0,3)",
			w:        PaymentWindow{Enabled: true, Location: lome, StartHour: 0, EndHour: 3},
			t:        mk(lome, 3, 0, 0),
			wantOpen: false,
		},
		{
			name:     "12h en journée, fenêtre [0,3)",
			w:        PaymentWindow{Enabled: true, Location: lome, StartHour: 0, EndHour: 3},
			t:        mk(lome, 12, 0, 0),
			wantOpen: false,
		},
		{
			name:     "Wrap 22h-3h, à 23h",
			w:        PaymentWindow{Enabled: true, Location: lome, StartHour: 22, EndHour: 3},
			t:        mk(lome, 23, 0, 0),
			wantOpen: true,
		},
		{
			name:     "Wrap 22h-3h, à 02h",
			w:        PaymentWindow{Enabled: true, Location: lome, StartHour: 22, EndHour: 3},
			t:        mk(lome, 2, 0, 0),
			wantOpen: true,
		},
		{
			name:     "Wrap 22h-3h, à 12h",
			w:        PaymentWindow{Enabled: true, Location: lome, StartHour: 22, EndHour: 3},
			t:        mk(lome, 12, 0, 0),
			wantOpen: false,
		},
		{
			name:     "Fenêtre vide (Start==End) toujours fermée",
			w:        PaymentWindow{Enabled: true, Location: lome, StartHour: 23, EndHour: 23},
			t:        mk(lome, 23, 0, 0),
			wantOpen: false,
		},
		{
			name: "23h UTC = 23h Lome (UTC+0), fenêtre [0,3) Lome → fermée",
			w:    PaymentWindow{Enabled: true, Location: lome, StartHour: 0, EndHour: 3},
			// Africa/Lome est UTC+0 toute l'année (pas de DST)
			t:        mk(time.UTC, 23, 0, 0),
			wantOpen: false,
		},
		{
			name:     "01h UTC = 01h Lome, fenêtre [0,3) Lome → ouverte",
			w:        PaymentWindow{Enabled: true, Location: lome, StartHour: 0, EndHour: 3},
			t:        mk(time.UTC, 1, 0, 0),
			wantOpen: true,
		},
		{
			name:     "Location nil retourne true (graceful)",
			w:        PaymentWindow{Enabled: true, Location: nil, StartHour: 0, EndHour: 3},
			t:        mk(time.UTC, 12, 0, 0),
			wantOpen: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.w.IsOpen(c.t)
			if got != c.wantOpen {
				t.Errorf("IsOpen(%s) = %v, want %v", c.t.Format(time.RFC3339), got, c.wantOpen)
			}
		})
	}
}
