package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/DefNotXavier/WeatherApp/server/auth"
	"github.com/DefNotXavier/WeatherApp/server/store"
	"golang.org/x/crypto/bcrypt"
)

const sessionCookie = "session"

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimSpace(r.PostFormValue("username"))
	password := strings.TrimSpace(r.PostFormValue("password"))

	hash, err := s.Store.PasswordHash(r.Context(), username)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		s.serverError(w, r, err)
		return
	}
	// PHP's password_hash() makes $2y$ bcrypt hashes, which Go's bcrypt reads fine
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		http.Redirect(w, r, "/?error=1", http.StatusSeeOther)
		return
	}

	token, err := s.Sessions.Create(r.Context(), username)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(auth.SessionTTL / time.Second),
		HttpOnly: true,
		Secure:   s.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/history", http.StatusSeeOther)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		if err := s.Sessions.Delete(r.Context(), c.Value); err != nil {
			s.Log.Warn("deleting session failed", "err", err)
		}
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/logged-out.html", http.StatusSeeOther)
}

type userKey struct{}

func withUser(ctx context.Context, username string) context.Context {
	return context.WithValue(ctx, userKey{}, username)
}

func userFrom(ctx context.Context) string {
	u, _ := ctx.Value(userKey{}).(string)
	return u
}

func (s *Server) requireLogin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		username, ok, err := s.Sessions.Lookup(r.Context(), c.Value)
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		if !ok {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		next(w, r.WithContext(withUser(r.Context(), username)))
	}
}

func (s *Server) historyPage(w http.ResponseWriter, r *http.Request) {
	searches, err := s.Store.RecentSearches(r.Context(), 500)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.history.Execute(w, map[string]any{
		"User":     userFrom(r.Context()),
		"Searches": searches,
	}); err != nil {
		s.Log.Error("rendering history failed", "err", err)
	}
}
