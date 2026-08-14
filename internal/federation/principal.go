package federation

import "time"

type Principal struct {
	Subject  string
	Audience string
	Scopes   []string
	AuthTime time.Time
	AMR      []string
}

func (p Principal) HasScope(required string) bool {
	if required == "" {
		return true
	}
	for _, scope := range p.Scopes {
		if scope == required {
			return true
		}
	}
	return false
}
