package domain

// UserFile représente un fichier associé au profil utilisateur.
// Stub — sera implémenté quand file-service sera prêt.
type UserFile struct {
	FileID   string `bson:"file_id"`
	FileURL  string `bson:"file_url"`
	FileType string `bson:"file_type"`
}
