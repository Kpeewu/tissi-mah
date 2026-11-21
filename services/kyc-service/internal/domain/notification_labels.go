package domain

// Libellés français lisibles pour les notifications utilisateur (push + email)
// envoyées après la revue d'un document.

// DocumentTypeLabel retourne un libellé lisible du type de document à partir du
// type physique (idCardFront, driverLicence, insurance, …). Le type logique
// est utilisé pour regrouper recto/verso sous un même libellé.
func DocumentTypeLabel(documentType string) string {
	switch ToLogicalDocumentType(documentType) {
	case "idCard":
		return "Carte d'identité"
	case "passport":
		return "Passeport"
	case "driverLicence":
		return "Permis de conduire"
	case "selfie":
		return "Selfie d'identité"
	case "insurance":
		return "Assurance"
	case "registrationCard":
		return "Carte grise"
	default:
		return documentType
	}
}

// rejectionReasonLabels mappe les codes de motif de rejet (cf. file-service
// ValidReasonRejections) vers un libellé français.
var rejectionReasonLabels = map[string]string{
	"document_expired":      "Document expiré",
	"document_incomplete":   "Document incomplet",
	"document_illegible":    "Document illisible",
	"photo_missmatch":       "La photo ne correspond pas",
	"information_missmatch": "Les informations ne correspondent pas",
	"wrong_document_type":   "Type de document incorrect",
	"other":                 "Autre motif",
}

// RejectionReasonText construit le motif affiché dans l'email de rejet : le
// libellé du motif standardisé, suivi du commentaire libre de l'agent s'il existe.
func RejectionReasonText(reason, details string) string {
	label := rejectionReasonLabels[reason]
	if label == "" {
		label = reason
	}
	if details != "" {
		if label != "" {
			return label + " — " + details
		}
		return details
	}
	return label
}
