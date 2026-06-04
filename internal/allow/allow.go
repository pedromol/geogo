package allow

import "strings"

type Rules struct {
	Continent map[string]struct{}
	Country   map[string]struct{}
	City      map[string]struct{}
}

func NewSetFromCSV(raw string) map[string]struct{} {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	out := make(map[string]struct{})
	for _, part := range strings.Split(raw, ",") {
		v := strings.ToLower(strings.TrimSpace(part))
		if v == "" {
			continue
		}
		out[v] = struct{}{}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalize(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func Check(continent, country, city string, rules Rules) bool {
	if rules.Continent != nil {
		v := normalize(continent)
		if v == "" {
			return false
		}
		if _, ok := rules.Continent[v]; !ok {
			return false
		}
	}

	if rules.Country != nil {
		v := normalize(country)
		if v == "" {
			return false
		}
		if _, ok := rules.Country[v]; !ok {
			return false
		}
	}

	if rules.City != nil {
		v := normalize(city)
		if v == "" {
			return false
		}
		if _, ok := rules.City[v]; !ok {
			return false
		}
	}

	return true
}
