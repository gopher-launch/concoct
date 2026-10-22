package codexapp

import "strings"

// finalResult gives protocol final_answer messages priority. Without one,
// only the last phase-less agent item can supply a compatibility result.
type finalResult struct {
	text     string
	explicit bool
}

func (r *finalResult) add(phase, value string) {
	if phase == "final_answer" {
		r.text, r.explicit = value, true
		return
	}
	if r.explicit || phase != "" {
		return
	}
	r.text = ""
	if strings.TrimSpace(value) == "" {
		return
	}
	if _, err := ParseRoleResult(value); err == nil {
		r.text = value
	}
}
