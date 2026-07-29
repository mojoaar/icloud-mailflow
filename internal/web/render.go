package web

import (
	"bytes"
	"embed"
	"html/template"
	"net/http"
)

//go:embed templates/*.html
var templatesFS embed.FS

type pageData struct {
	Title   string
	Content template.HTML
}

var tmpl *template.Template

func init() {
	tmpl = template.Must(template.New("").ParseFS(templatesFS, "templates/*.html"))
}

func renderPage(w http.ResponseWriter, _ *http.Request, title string, pageName string, data any) {
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, pageName, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	pd := pageData{
		Title:   title,
		Content: template.HTML(buf.String()),
	}
	if err := tmpl.ExecuteTemplate(w, "base.html", pd); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func renderPartial(w http.ResponseWriter, pageName string, data any) {
	tmpl.ExecuteTemplate(w, pageName, data)
}
