package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/DefNotXavier/WeatherApp/server/geo"
	"github.com/DefNotXavier/WeatherApp/server/store"
	"github.com/DefNotXavier/WeatherApp/server/weather"
	"golang.org/x/crypto/bcrypt"
)

// fakeStore snaps to the closest of the test graph's nodes and remembers
// what got logged.
type fakeStore struct {
	nodes  []store.Snapped
	logged []store.Search
	hash   string
}

func (f *fakeStore) Ping(context.Context) error { return nil }
func (f *fakeStore) HighwaysGeoJSON(context.Context) ([]byte, error) {
	return []byte(`{"type":"FeatureCollection","features":[]}`), nil
}
func (f *fakeStore) BorderGeoJSON(ctx context.Context) ([]byte, error) { return f.HighwaysGeoJSON(ctx) }
func (f *fakeStore) POIsGeoJSON(ctx context.Context) ([]byte, error)   { return f.HighwaysGeoJSON(ctx) }
func (f *fakeStore) NearestNode(_ context.Context, p geo.Point) (store.Snapped, error) {
	best := f.nodes[0]
	for _, n := range f.nodes {
		if geo.DistanceMiles(p, n.Point) < geo.DistanceMiles(p, best.Point) {
			best = n
		}
	}
	return best, nil
}
func (f *fakeStore) POIsNearRoute(context.Context, []geo.Point, float64) ([]store.POI, error) {
	return []store.POI{{ID: 1, Name: "Butte", Type: "city"}}, nil
}
func (f *fakeStore) LogSearch(_ context.Context, s store.Search) error {
	f.logged = append(f.logged, s)
	return nil
}
func (f *fakeStore) RecentSearches(context.Context, int) ([]store.Search, error) {
	return f.logged, nil
}
func (f *fakeStore) PasswordHash(_ context.Context, u string) (string, error) {
	if u != "admin" {
		return "", store.ErrNotFound
	}
	return f.hash, nil
}

type fakeSessions map[string]string

func (f fakeSessions) Create(_ context.Context, u string) (string, error) {
	f["tok"] = u
	return "tok", nil
}
func (f fakeSessions) Lookup(_ context.Context, t string) (string, bool, error) {
	u, ok := f[t]
	return u, ok, nil
}
func (f fakeSessions) Delete(_ context.Context, t string) error { delete(f, t); return nil }
func (f fakeSessions) Ping(context.Context) error               { return nil }

type fakeWeather struct{ calls int }

func (f *fakeWeather) Forecast(context.Context, float64, float64) ([]weather.Period, bool, error) {
	f.calls++
	return []weather.Period{{Name: "Today", Temperature: 40}}, f.calls > 1, nil
}

func testServer(t *testing.T) (*Server, *fakeStore, http.Handler) {
	// a 2-highway network from Missoula east to Billings, then on to Glendive
	vertices := []geo.Vertex{
		{Highway: "I-90", Seq: 1, Point: geo.Point{Lat: 46.87, Lng: -113.99}},
		{Highway: "I-90", Seq: 2, Point: geo.Point{Lat: 45.78, Lng: -108.50}},
		{Highway: "I-94", Seq: 1, Point: geo.Point{Lat: 45.78, Lng: -108.50}},
		{Highway: "I-94", Seq: 2, Point: geo.Point{Lat: 47.10, Lng: -104.71}},
	}
	fs := &fakeStore{}
	for _, v := range vertices {
		fs.nodes = append(fs.nodes, store.Snapped{Point: v.Point, Highway: v.Highway})
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.MinCost)
	fs.hash = string(hash)

	s := &Server{
		Store:           fs,
		Graph:           geo.NewGraph(vertices),
		Weather:         &fakeWeather{},
		Sessions:        fakeSessions{},
		Log:             slog.New(slog.NewTextHandler(io.Discard, nil)),
		WebDir:          t.TempDir(),
		CheckpointMiles: 15,
		NearbyMiles:     5,
	}
	return s, fs, s.Handler()
}

func TestPlanRoute(t *testing.T) {
	_, fs, h := testServer(t)

	body := `{"from":{"lat":46.9,"lng":-114.0},"to":{"lat":47.1,"lng":-104.8}}`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/api/routes", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}

	var resp routeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if strings.Join(resp.Highways, ",") != "I-90,I-94" {
		t.Fatalf("highways = %v", resp.Highways)
	}
	if resp.Line.Type != "LineString" || len(resp.Line.Coordinates) != 3 {
		t.Fatalf("line = %+v", resp.Line)
	}
	// GeoJSON coordinates are [lng, lat]
	if resp.Line.Coordinates[0] != [2]float64{-113.99, 46.87} {
		t.Fatalf("first coordinate = %v, want [lng, lat]", resp.Line.Coordinates[0])
	}
	if n := len(resp.Checkpoints); n < 30 {
		t.Fatalf("got %d checkpoints for a ~480 mile route, want one every 15 miles", n)
	}
	if len(fs.logged) != 1 || fs.logged[0].Checkpoints != len(resp.Checkpoints) {
		t.Fatalf("search should be logged once, got %+v", fs.logged)
	}
}

func TestPlanRouteRejectsBadInput(t *testing.T) {
	_, fs, h := testServer(t)
	for _, body := range []string{
		`not json`,
		`{"from":{"lat":40.7,"lng":-74.0},"to":{"lat":47.1,"lng":-104.8}}`, // New York
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("POST", "/api/routes", strings.NewReader(body)))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", body, rec.Code)
		}
	}
	if len(fs.logged) != 0 {
		t.Fatal("bad requests should not be logged")
	}
}

func TestWeather(t *testing.T) {
	_, _, h := testServer(t)
	for i, want := range []string{"MISS", "HIT"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/weather?lat=46.87&lng=-113.99", nil))
		if rec.Code != http.StatusOK || rec.Header().Get("X-Cache") != want {
			t.Fatalf("request %d: status %d, X-Cache %q, want 200 %s", i+1, rec.Code, rec.Header().Get("X-Cache"), want)
		}
	}
	for _, q := range []string{"", "lat=abc&lng=1", "lat=40.7&lng=-74.0"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/weather?"+q, nil))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%q: status %d, want 400", q, rec.Code)
		}
	}
}

func TestLoginAndHistory(t *testing.T) {
	_, _, h := testServer(t)

	// history needs a login
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/history", nil))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("logged out /history: %d %s", rec.Code, rec.Header().Get("Location"))
	}

	login := func(user, pass string) *httptest.ResponseRecorder {
		form := url.Values{"username": {user}, "password": {pass}}
		req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	for _, c := range [][2]string{{"admin", "wrong"}, {"nobody", "admin123"}} {
		if rec := login(c[0], c[1]); rec.Header().Get("Location") != "/?error=1" {
			t.Fatalf("login %v should fail, redirected to %s", c, rec.Header().Get("Location"))
		}
	}

	rec = login("admin", "admin123")
	if rec.Header().Get("Location") != "/history" {
		t.Fatalf("good login redirected to %s", rec.Header().Get("Location"))
	}
	cookie := rec.Result().Cookies()[0]
	if !cookie.HttpOnly {
		t.Fatal("session cookie should be HttpOnly")
	}

	req := httptest.NewRequest("GET", "/history", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Montana Route Planner — admin") {
		t.Fatalf("history page: %d", rec.Code)
	}
}
