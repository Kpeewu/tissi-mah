package gateway

import "testing"

func TestRouteLookup_Matches(t *testing.T) {
	lookup := NewRouteLookup(ProtectedRoutes)

	cases := []struct {
		name string
		path string
		want bool
	}{
		// Routes exactes protégées
		{"exact user/me", "/api/v1/user/me", true},
		{"exact notifications inbox", "/api/v1/notifications/inbox", true},
		{"exact chat threads", "/api/v1/chat/threads", true},

		// Routes templated : le paramètre de chemin doit matcher
		{"templated notif read", "/api/v1/notifications/inbox/notif-uuid-001/read", true},
		{"templated chat messages", "/api/v1/chat/threads/thread-123/messages", true},
		{"templated chat read", "/api/v1/chat/threads/thread-123/read", true},
		{"templated chat flag", "/api/v1/chat/messages/msg-9/flag", true},

		// Routes publiques → non protégées
		{"public notifications health", "/api/v1/notifications/health", false},
		{"public chat health", "/api/v1/chat/health", false},
		{"unknown route", "/api/v1/does/not/exist", false},

		// Pas de faux positif : nombre de segments différent
		{"readAll not matched by {InboxId}/read", "/api/v1/notifications/inbox/readAll", true}, // exact protégé
		{"extra segment under templated", "/api/v1/notifications/inbox/a/b/read", false},
		{"missing trailing segment", "/api/v1/chat/threads/thread-123", false},

		// Support-only route ne doit pas matcher dans ProtectedRoutes
		{"support flagged-content not in ProtectedRoutes", "/api/v1/chat/messages/msg-9/flagged-content", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := lookup.Matches(c.path); got != c.want {
				t.Errorf("Matches(%q) = %v, want %v", c.path, got, c.want)
			}
		})
	}
}

func TestRouteLookup_NoWildcardCrossMatch(t *testing.T) {
	// Un pattern ne doit matcher que si les segments littéraux coïncident.
	lookup := NewRouteLookup(map[string]bool{
		"/api/v1/chat/threads/{ThreadId}/read": true,
	})

	if lookup.Matches("/api/v1/chat/messages/msg-1/read") {
		t.Error("path avec segment littéral différent (messages vs threads) ne devrait pas matcher")
	}
	if !lookup.Matches("/api/v1/chat/threads/t-1/read") {
		t.Error("path avec bon segment littéral devrait matcher")
	}
}

func TestRouteLookup_TierValue(t *testing.T) {
	lookup := NewRouteLookup(RouteRateLimitConfig)

	// Route exacte
	if tier, ok := lookup.Lookup("/api/v1/auth/createAccount"); !ok || tier != TierCreateAccount {
		t.Errorf("createAccount tier = %v, ok=%v ; want %v, true", tier, ok, TierCreateAccount)
	}

	// Route templated : le tier doit être retourné pour un vrai path
	if tier, ok := lookup.Lookup("/api/v1/chat/messages/msg-1/flag"); !ok || tier != TierSensitive {
		t.Errorf("chat flag tier = %v, ok=%v ; want %v, true", tier, ok, TierSensitive)
	}
	if tier, ok := lookup.Lookup("/api/v1/chat/threads/t-1/messages"); !ok || tier != TierCreateAccount {
		t.Errorf("chat messages tier = %v, ok=%v ; want %v, true", tier, ok, TierCreateAccount)
	}

	// Route sans tier configuré → non trouvée (tombe sur global côté appelant)
	if _, ok := lookup.Lookup("/api/v1/user/me"); ok {
		t.Error("user/me ne devrait pas avoir de tier configuré")
	}
}
