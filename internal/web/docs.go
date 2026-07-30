package web

import "net/http"

func docsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		renderPage(w, r, "Documentation", "docs", map[string]any{"Host": r.Host})
	}
}
