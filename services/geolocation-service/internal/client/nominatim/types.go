package nominatim

// nominatimSearchResult correspond à un élément de la réponse JSON de
// l'endpoint /search?format=json de Nominatim. On ne décode que les champs
// utilisés par geolocation-service.
type nominatimSearchResult struct {
	Lat         string             `json:"lat"`
	Lon         string             `json:"lon"`
	DisplayName string             `json:"display_name"`
	Type        string             `json:"type"`
	Class       string             `json:"class"`
	Address     *nominatimAddress  `json:"address,omitempty"`
}

// nominatimReverseResult correspond à la réponse JSON de l'endpoint
// /reverse?format=json (objet unique, pas un tableau).
type nominatimReverseResult struct {
	Lat         string            `json:"lat"`
	Lon         string            `json:"lon"`
	DisplayName string            `json:"display_name"`
	Type        string            `json:"type"`
	Class       string            `json:"class"`
	Address     *nominatimAddress `json:"address,omitempty"`
	Error       string            `json:"error,omitempty"` // présent quand pas de match
}

// nominatimAddress contient les sous-champs d'adresse renvoyés par Nominatim
// quand addressdetails=1. Les champs sont best-effort (Nominatim renvoie un
// sous-ensemble selon le type d'adresse — ville, village, hameau, etc.).
type nominatimAddress struct {
	City        string `json:"city,omitempty"`
	Town        string `json:"town,omitempty"`
	Village     string `json:"village,omitempty"`
	Country     string `json:"country,omitempty"`
	CountryCode string `json:"country_code,omitempty"` // ISO 3166-1 alpha-2 lowercase
}

// pickCity retourne le nom de la ville/village le plus précis disponible.
func (a *nominatimAddress) pickCity() string {
	if a == nil {
		return ""
	}
	if a.City != "" {
		return a.City
	}
	if a.Town != "" {
		return a.Town
	}
	return a.Village
}
