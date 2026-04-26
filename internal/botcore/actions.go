package botcore

import "strings"

const (
	ActionPrefixSource    = "src:"
	ActionPrefixMfr       = "mfr:"
	ActionPrefixModel     = "mdl:"
	ActionPrefixEngine    = "eng:"
	ActionPrefixMaxKm     = "maxkm:"
	ActionPrefixMaxHand   = "maxhand:"
	ActionConfirm         = "confirm:yes"
	ActionStartOver       = "confirm:restart"
	ActionCancel          = "confirm:cancel"
	ActionSkipKeywords    = "skip_keywords"
	ActionSkipExcludeKeys = "skip_exclude_keys"
	ActionSourceToggle    = "src_toggle:"
	ActionSourceDone      = "src_done"
	ActionMfrSearch       = "mfr_search"
	ActionAnyModel        = "mdl:0"
	ActionPrefixSave      = "save:"
	ActionPrefixHide      = "hide:"
)

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
