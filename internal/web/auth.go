package web

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/mojoaar/icloud-mailflow/internal/crypto"
	"github.com/mojoaar/icloud-mailflow/internal/db"
)

const sessionCookie = "mailflow_session"
const sessionTTL = 24 * time.Hour

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func loginPage(settingsRepo *db.SettingsRepo, sessRepo *db.SessionsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			renderPage(w, r, "Login", "login", map[string]any{})
			return
		}
		ip := r.RemoteAddr
		if !loginLimiter.allow(ip, 5, time.Minute) {
			renderPage(w, r, "Login", "login", map[string]any{"Error": "Too many attempts. Wait a minute."})
			return
		}
		r.ParseForm()
		password := r.FormValue("password")
		hash, _ := settingsRepo.Get("admin_password_hash")
		if hash == "" || !crypto.CheckPassword(hash, password) {
			renderPage(w, r, "Login", "login", map[string]string{"Error": "Invalid password"})
			return
		}
		token, err := generateToken()
		if err != nil {
			renderPage(w, r, "Login", "login", map[string]string{"Error": "Internal error. Try again."})
			return
		}
		sessRepo.Create(token, sessionTTL)
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookie,
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			Secure:   false,
			SameSite: http.SameSiteStrictMode,
			Expires:  time.Now().Add(sessionTTL),
		})
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	}
}

func logoutHandler(sessRepo *db.SessionsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookie)
		if err == nil {
			sessRepo.Delete(cookie.Value)
		}
		http.SetCookie(w, &http.Cookie{
			Name:   sessionCookie,
			Value:  "",
			Path:   "/",
			MaxAge: -1,
		})
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}

func authMiddleware(sessRepo *db.SessionsRepo) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/login") ||
				strings.HasPrefix(r.URL.Path, "/setup") ||
				strings.HasPrefix(r.URL.Path, "/mcp") ||
				strings.HasPrefix(r.URL.Path, "/docs") ||
				strings.HasPrefix(r.URL.Path, "/health") ||
				strings.HasPrefix(r.URL.Path, "/static/") {
				next.ServeHTTP(w, r)
				return
			}
			cookie, err := r.Cookie(sessionCookie)
			if err != nil {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			valid, _ := sessRepo.Validate(cookie.Value)
			if !valid {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
