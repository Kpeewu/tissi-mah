package implementations

// anonymizedUserID retourne le pseudonyme stocké à la place de userID ; tolère les IDs de moins de 8 caractères.
func anonymizedUserID(userID string) string {
	const prefixLen = 8
	if len(userID) > prefixLen {
		userID = userID[:prefixLen]
	}
	return "deleted_" + userID
}
