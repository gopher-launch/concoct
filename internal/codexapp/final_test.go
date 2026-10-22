package codexapp

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestFinalResultPrecedence(t *testing.T) {
	good := `{"disposition":"ready_for_review","summary":"done"}`
	cases := []struct {
		name  string
		items [][2]string
		want  string
	}{
		{"explicit with surrounding commentary", [][2]string{{"commentary", "before"}, {"final_answer", good}, {"commentary", "after"}}, good},
		{"explicit replaces absent", [][2]string{{"", good}, {"final_answer", "{bad"}, {"", good}}, "{bad"},
		{"null compatibility", [][2]string{{"", "commentary"}, {"", good}}, good},
		{"intermediate null result followed by prose", [][2]string{{"", good}, {"", "No response is needed."}}, ""},
		{"intermediate null result followed by empty item", [][2]string{{"", good}, {"", ""}}, ""},
		{"commentary cannot supply result", [][2]string{{"commentary", good}}, ""},
		{"malformed null is not result", [][2]string{{"", "{bad"}}, ""},
		{"empty explicit masks fallback", [][2]string{{"", good}, {"final_answer", ""}}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var result finalResult
			for _, item := range tc.items {
				result.add(item[0], item[1])
			}
			if result.text != tc.want {
				t.Fatalf("got %q, want %q", result.text, tc.want)
			}
		})
	}
}

func TestRoleResultSchemaAndMalformedResponse(t *testing.T) {
	if !json.Valid(RoleResultSchema) {
		t.Fatal("role result schema is not JSON")
	}
	for _, value := range []string{`{"disposition":"approved","summary":"checked","unknown":"ignored?"}`, `{"disposition":"invented","summary":"done"}`, `{"disposition":"approved","summary":"done"} trailing`} {
		if _, err := ParseRoleResult(value); !errors.Is(err, ErrMalformedRoleResult) {
			t.Fatalf("invalid role result %q accepted: %v", value, err)
		}
	}
}
