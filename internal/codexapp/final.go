package codexapp

import "strings"

// finalResult gives protocol final_answer messages priority over older
// phase-less structured results. An explicit but malformed final answer still
// wins, so a compatibility result cannot conceal it.
type finalResult struct {
	text     string
	explicit bool
}

func (r *finalResult) add(phase, value string) {
	if phase == "final_answer" {
		r.text, r.explicit = value, true
		return
	}
	if r.explicit || phase != "" || strings.TrimSpace(value) == "" {
		return
	}
	if _, err := ParseRoleResult(value); err == nil {
		r.text = value
	}
}
