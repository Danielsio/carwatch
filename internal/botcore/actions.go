package botcore

import "strings"

func NormalizeKeywords(input string) string {
	parts := strings.Split(input, ",")
	var trimmed []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			trimmed = append(trimmed, p)
		}
	}
	return strings.Join(trimmed, ",")
}

func ToggleSource(current, toggle string) string {
	sources := make(map[string]bool)
	if current != "" {
		for _, s := range strings.Split(current, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				sources[s] = true
			}
		}
	}
	toggle = strings.TrimSpace(toggle)
	if toggle != "" {
		if sources[toggle] {
			delete(sources, toggle)
		} else {
			sources[toggle] = true
		}
	}
	var result []string
	for _, s := range []string{"yad2", "winwin"} {
		if sources[s] {
			result = append(result, s)
		}
	}
	return strings.Join(result, ",")
}
