package templateutils

import (
	"bytes"
	"text/template"
)

// String executes a template and returns the result as a string.
func String(t *template.Template, data interface{}) (string, error) {
	var buf = new(bytes.Buffer)
	err := t.Execute(buf, data)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
