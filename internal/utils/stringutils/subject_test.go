package stringutils

import "testing"

func TestParseSubjectBody(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		subject string
		body    string
	}{
		{"empty", "", "", ""},
		{"subject only", "subject", "subject", ""},
		{"subject and body", "subject\n\n\nbody\n\n", "subject", "body"},
		{
			"subject and body with newlines",
			"subject\n\n\nbody\nmore body\n",
			"subject",
			"body\nmore body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subject, body := ParseSubjectBody(tt.input)
			if subject != tt.subject {
				t.Errorf("subject = %q, want %q", subject, tt.subject)
			}
			if body != tt.body {
				t.Errorf("body = %q, want %q", body, tt.body)
			}
		})
	}
}
