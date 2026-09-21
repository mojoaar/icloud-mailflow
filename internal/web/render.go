package web

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
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
	Nonce     string
	Page      string
	ShowNav   bool
	Version   string
	MonoFont  bool
}

var tmpl *template.Template

var appVersion string
var buildCommit = "dev"
var useMonoFont atomic.Bool
var startTime time.Time

var templateFuncs = template.FuncMap{
	"percent": func(val, max int) int {
		if max == 0 {
			return 0
		}
		return val * 100 / max
	},
	"subtract":        func(a, b int) int { return a - b },
	"add":             func(a, b int) int { return a + b },
	"hasPrefix":       strings.HasPrefix,
	"trimPrefix":      strings.TrimPrefix,
	"groupViews":      buildGroupViews,
	"rootGroupView":   rootGroupView,
	"totalConditions": totalConditions,
	"navMatch":        navMatch,
	"formatCPU": func(v int) string {
		pct := float64(v) / 10000.0
		return fmt.Sprintf("%.1f%%", pct)
	},
	"hasDay": func(days []string, day string) bool {
		for _, d := range days {
			if d == day {
				return true
			}
		}
		return false
	},
}

// navMatch reports whether the current page belongs to a nav section.
func navMatch(page, section string) bool {
	switch section {
	case "dashboard":
		return page == "dashboard"
	case "activity":
		return page == "activity"
	case "rules":
		return page == "rules" || page == "rules_list" || page == "rules_form"
	case "settings":
		return page == "settings"
	case "stats":
		return page == "stats"
	}
	return false
}

func formatUptime(d time.Duration) string {
	d = d.Truncate(time.Second)
	if d < 0 {
		d = 0
	}
	days := d / (24 * time.Hour)
	d -= days * 24 * time.Hour
	hours := d / time.Hour
	d -= hours * time.Hour
	mins := d / time.Minute
	d -= mins * time.Minute
	secs := d / time.Second

	var parts []string
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if days > 0 || hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if days > 0 || hours > 0 || mins > 0 {
		parts = append(parts, fmt.Sprintf("%dm", mins))
	}
	parts = append(parts, fmt.Sprintf("%ds", secs))
	return strings.Join(parts, " ")
}

func init() {
	tmpl = template.Must(template.New("").Funcs(templateFuncs).ParseFS(templatesFS, "templates/*.html"))
}

func renderPage(w http.ResponseWriter, r *http.Request, title string, pageName string, data any) {
	token, err := csrfTokenForRequest(r)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, csrfCookieWithToken(token, r))
	nonce := nonceFrom(r)
	if m, ok := data.(map[string]any); ok {
		m["CSRFToken"] = token
		m["Nonce"] = nonce
	} else {
		data = map[string]any{"CSRFToken": token, "Nonce": nonce}
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
		Nonce:     nonce,
		Page:      pageName,
		ShowNav:   pageName != "login" && pageName != "setup",
		Version:   appVersion,
		MonoFont:  useMonoFont.Load(),
	}
	if err := tmpl.ExecuteTemplate(w, "base.html", pd); err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
	}
}

func renderPartial(w http.ResponseWriter, pageName string, data any) {
	if err := tmpl.ExecuteTemplate(w, pageName, data); err != nil {
		slog.Error("render partial failed", "template", pageName, "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
	}
}
