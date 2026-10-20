// Package api is the HTTP side of the app: JSON endpoints for the map, the
// admin login, and the route history page.
package api

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/DefNotXavier/WeatherApp/server/geo"
	"github.com/DefNotXavier/WeatherApp/server/store"
	"github.com/DefNotXavier/WeatherApp/server/weather"
)

//go:embed templates/*.html
var templates embed.FS

// Store is the part of store.Store the handlers use, so tests can swap in a fake.
type Store interface {
	Ping(ctx context.Context) error
	HighwaysGeoJSON(ctx context.Context) ([]byte, error)
	BorderGeoJSON(ctx context.Context) ([]byte, error)
	POIsGeoJSON(ctx context.Context) ([]byte, error)
	NearestNode(ctx context.Context, p geo.Point) (store.Snapped, error)
	POIsNearRoute(ctx context.Context, route []geo.Point, miles float64) ([]store.POI, error)
	LogSearch(ctx context.Context, s store.Search) error
	RecentSearches(ctx context.Context, limit int) ([]store.Search, error)
	PasswordHash(ctx context.Context, username string) (string, error)
}

type Sessions interface {
	Create(ctx context.Context, username string) (string, error)
	Lookup(ctx context.Context, token string) (string, bool, error)
	Delete(ctx context.Context, token string) error
	Ping(ctx context.Context) error
}

type Forecaster interface {
	Forecast(ctx context.Context, lat, lng float64) ([]weather.Period, bool, error)
}

type Server struct {
	Store    Store
	Graph    *geo.Graph
	Weather  Forecaster
	Sessions Sessions
	Log      *slog.Logger
	WebDir   string

	// how far apart weather checkpoints are, and how close a point of
	// interest has to be to count as "along the route"
	CheckpointMiles float64
	NearbyMiles     float64
	SecureCookies   bool

	history *template.Template
}

func (s *Server) Handler() http.Handler {
	s.history = template.Must(template.New("history.html").Funcs(template.FuncMap{
		"badge": badgeColor,
		"inc":   func(i int) int { return i + 1 },
	}).ParseFS(templates, "templates/history.html"))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/highways", s.geojson(s.Store.HighwaysGeoJSON))
	mux.HandleFunc("GET /api/border", s.geojson(s.Store.BorderGeoJSON))
	mux.HandleFunc("GET /api/pois", s.geojson(s.Store.POIsGeoJSON))
	mux.HandleFunc("POST /api/routes", s.planRoute)
	mux.HandleFunc("GET /api/weather", s.weather)
	mux.HandleFunc("GET /healthz", s.health)

	mux.HandleFunc("POST /login", s.login)
	mux.HandleFunc("GET /logout", s.logout)
	mux.HandleFunc("GET /history", s.requireLogin(s.historyPage))

	mux.Handle("GET /", http.FileServer(http.Dir(s.WebDir)))
	return s.logRequests(mux)
}

func (s *Server) geojson(get func(context.Context) ([]byte, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, err := get(r.Context())
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		w.Header().Set("Content-Type", "application/geo+json")
		// the map data only changes when the database is reseeded
		w.Header().Set("Cache-Control", "public, max-age=300")
		w.Write(b)
	}
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.Store.Ping(ctx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	if err := s.Sessions.Ping(ctx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "redis unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "nodes": s.Graph.Len()})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (s *Server) serverError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, context.Canceled) {
		return
	}
	s.Log.Error("request failed", "path", r.URL.Path, "err", err)
	writeError(w, http.StatusInternalServerError, "something went wrong")
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.Log.Info("request", "method", r.Method, "path", r.URL.Path, "status", rec.status,
			"ms", time.Since(start).Milliseconds())
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func badgeColor(highway string) string {
	switch highway {
	case "I-90":
		return "blue"
	case "I-15":
		return "red"
	case "I-94":
		return "green"
	}
	return "gray"
}
