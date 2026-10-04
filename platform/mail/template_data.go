package mail

// TemplateData carries the fields used by mail templates. html/template
// auto-escapes every field; the reminder extras are zero-value safe for the
// reset_password / payment_link / verify_email templates.
type TemplateData struct {
	AppName       string
	Name          string
	URL           string
	InvoiceNumber string
	Total         string
	Currency      string
	DueDate       string
	Kind          string
}
