package locale

import "fmt"

type Lang string

const (
	Hebrew  Lang = "he"
	English Lang = "en"
)

func T(lang Lang, key string) string {
	if m, ok := translations[lang]; ok {
		if s, ok := m[key]; ok {
			return s
		}
	}
	if s, ok := translations[English][key]; ok {
		return s
	}
	return key
}

func Tf(lang Lang, key string, args ...any) string {
	return fmt.Sprintf(T(lang, key), args...)
}

var translations map[Lang]map[string]string

func init() {
	he := mergeMaps(heWizard, heCommands, heScoring)
	en := mergeMaps(enWizard, enCommands, enScoring)
	translations = map[Lang]map[string]string{
		Hebrew:  he,
		English: en,
	}
}
