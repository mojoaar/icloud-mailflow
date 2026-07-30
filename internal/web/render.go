package web

import (
	"bytes"
	"embed"
	"html/template"
	"net/http"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static
var staticFS embed.FS

type pageData struct {
	Title     string
	Content   template.HTML
	CSRFToken string
	ShowNav   bool
	Version   string
	MonoFont  bool
}

var tmpl *template.Template

var appVersion string
var useMonoFont bool

func init() {
	tmpl = template.Must(template.New("").ParseFS(templatesFS, "templates/*.html"))
}

func renderPage(w http.ResponseWriter, r *http.Request, title string, pageName string, data any) {
	token := csrfToken()
	http.SetCookie(w, csrfCookieWithToken(token))
	if m, ok := data.(map[string]any); ok {
		m["CSRFToken"] = token
	} else {
		data = map[string]any{"CSRFToken": token}
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, pageName, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	pd := pageData{
		Title:     title,
		Content:   template.HTML(buf.String()),
		CSRFToken: token,
		ShowNav:   pageName != "login" && pageName != "setup",
		Version:   appVersion,
		MonoFont:  useMonoFont,
	}
	if err := tmpl.ExecuteTemplate(w, "base.html", pd); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func renderPartial(w http.ResponseWriter, pageName string, data any) {
	tmpl.ExecuteTemplate(w, pageName, data)
}
