package mail

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
)

//go:embed templates/*.html
var templateFS embed.FS

var parsed = template.Must(template.ParseFS(templateFS, "templates/*.html"))

// Render executes one embedded template (reset_password, payment_link)
// with data. html/template auto-escapes every field.
func Render(name string, data TemplateData) (string, error) {
	var buf bytes.Buffer
	if err := parsed.ExecuteTemplate(&buf, name+".html", data); err != nil {
		return "", fmt.Errorf("mail template %q: %w", name, err)
	}
	return buf.String(), nil
}
