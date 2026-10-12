package implementations

import "testing"

func TestAnonymizedUserID(t *testing.T) {
	tests := []struct {
		name   string
		userID string
		want   string
	}{
		{"uuid tronqué à 8 caractères", "3f2a9c1e-7b4d-4e8a-9f10-2c3d4e5f6a7b", "deleted_3f2a9c1e"},
		{"exactement 8 caractères", "abcdefgh", "deleted_abcdefgh"},
		{"moins de 8 caractères", "abc", "deleted_abc"},
		{"chaîne vide", "", "deleted_"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := anonymizedUserID(tt.userID); got != tt.want {
				t.Errorf("anonymizedUserID(%q) = %q, want %q", tt.userID, got, tt.want)
			}
		})
	}
}
