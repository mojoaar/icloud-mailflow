package web

import (
	"bytes"
	"embed"
	"html/template"
	"net/http"
	"sync/atomic"
	"time"
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
var useMonoFont atomic.Bool
var startTime time.Time

var templateFuncs = template.FuncMap{
	"percent": func(val, max int) int {
		if max == 0 {
			return 0
		}
		return val * 100 / max
	},
	"subtract": func(a, b int) int { return a - b },
	"add":      func(a, b int) int { return a + b },
}

func init() {
	tmpl = template.Must(template.New("").Funcs(templateFuncs).ParseFS(templatesFS, "templates/*.html"))
}

func renderPage(w http.ResponseWriter, r *http.Request, title string, pageName string, data any) {
	token, err := csrfToken()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, csrfCookieWithToken(token, r))
	if m, ok := data.(map[string]any); ok {
		m["CSRFToken"] = token
	} else {
		data = map[string]any{"CSRFToken": token}
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, pageName, data); err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	pd := pageData{
		Title:     title,
		Content:   template.HTML(buf.String()),
		CSRFToken: token,
		ShowNav:   pageName != "login" && pageName != "setup",
		Version:   appVersion,
		MonoFont:  useMonoFont.Load(),
	}
	if err := tmpl.ExecuteTemplate(w, "base.html", pd); err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
	}
}

func renderPartial(w http.ResponseWriter, pageName string, data any) {
	tmpl.ExecuteTemplate(w, pageName, data)
}
