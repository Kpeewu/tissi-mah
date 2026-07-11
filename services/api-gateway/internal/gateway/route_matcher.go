package gateway

import "strings"

// RouteLookup matche un path HTTP entrant contre un ensemble de routes qui
// peuvent contenir des paramètres de chemin ({inbox_id}, {thread_id}...).
//
// Les prédicats de protection de l'api-gateway ne peuvent pas se contenter d'un
// lookup exact dans une map : une vraie requête (`/api/v1/notifications/inbox/
// abc-123/read`) ne matcherait jamais la clé templated
// (`/api/v1/notifications/inbox/{inbox_id}/read`) et la route serait traitée
// comme publique. RouteLookup résout ça en découpant chaque clé templated en
// segments et en traitant tout segment `{...}` comme un wildcard mono-segment.
//
// Le type est générique pour servir aussi bien aux maps de protection
// (map[string]bool) qu'à la config de rate limiting (map[string]RateLimitTier).
type RouteLookup[T any] struct {
	exact    map[string]T      // routes sans paramètre → lookup O(1)
	patterns []routePattern[T] // routes templated, découpées en segments
}

// routePattern représente une route templated : ses segments (dont certains
// sont des wildcards `{...}`) et la valeur associée.
type routePattern[T any] struct {
	segs  []string
	value T
}

// NewRouteLookup construit un RouteLookup à partir d'une map de routes. Les clés
// contenant un paramètre `{...}` sont rangées comme patterns segmentés, les
// autres comme entrées exactes.
func NewRouteLookup[T any](routes map[string]T) *RouteLookup[T] {
	m := &RouteLookup[T]{exact: make(map[string]T)}
	for route, value := range routes {
		if strings.Contains(route, "{") {
			m.patterns = append(m.patterns, routePattern[T]{
				segs:  splitPath(route),
				value: value,
			})
		} else {
			m.exact[route] = value
		}
	}
	return m
}

// Lookup retourne la valeur associée au path et true s'il matche une route
// (exacte ou templated), sinon la valeur zéro et false. Les routes exactes sont
// testées en premier (O(1)), puis les patterns.
func (m *RouteLookup[T]) Lookup(path string) (T, bool) {
	if v, ok := m.exact[path]; ok {
		return v, true
	}
	if len(m.patterns) > 0 {
		segs := splitPath(path)
		for i := range m.patterns {
			if matchSegments(m.patterns[i].segs, segs) {
				return m.patterns[i].value, true
			}
		}
	}
	var zero T
	return zero, false
}

// Matches indique si le path matche une route (exacte ou templated).
func (m *RouteLookup[T]) Matches(path string) bool {
	_, ok := m.Lookup(path)
	return ok
}

// splitPath découpe un path en segments, en ignorant les slashes de bordure.
func splitPath(path string) []string {
	return strings.Split(strings.Trim(path, "/"), "/")
}

// matchSegments compare les segments d'un pattern à ceux d'un path : même
// nombre de segments, et chaque segment de pattern doit être égal au segment du
// path — sauf les segments `{...}` qui matchent n'importe quelle valeur.
func matchSegments(pattern, path []string) bool {
	if len(pattern) != len(path) {
		return false
	}
	for i, p := range pattern {
		if strings.HasPrefix(p, "{") && strings.HasSuffix(p, "}") {
			continue // wildcard mono-segment
		}
		if p != path[i] {
			return false
		}
	}
	return true
}
