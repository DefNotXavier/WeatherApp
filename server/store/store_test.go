package store

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/DefNotXavier/WeatherApp/server/geo"
)

// These run against the real PostGIS database:
//
//	docker compose up -d
//	TEST_DATABASE_URL=postgres://montana:dev_db_pw@localhost:5433/montana_planner go test ./...
func testStore(t *testing.T) *Store {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping database tests")
	}
	s, err := Open(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

var (
	missoula = geo.Point{Lat: 46.8721, Lng: -113.9940}
	billings = geo.Point{Lat: 45.7833, Lng: -108.5007}
	helena   = geo.Point{Lat: 46.5958, Lng: -112.0270}
	glendive = geo.Point{Lat: 47.1053, Lng: -104.7124}
)

func TestGeoJSON(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	var fc struct {
		Features []struct {
			Geometry struct {
				Type string `json:"type"`
			} `json:"geometry"`
		} `json:"features"`
	}
	for _, c := range []struct {
		name  string
		get   func(context.Context) ([]byte, error)
		count int
		geom  string
	}{
		{"highways", s.HighwaysGeoJSON, 3, "LineString"},
		{"border", s.BorderGeoJSON, 1, "Polygon"},
		{"pois", s.POIsGeoJSON, 41, "Point"},
	} {
		b, err := c.get(ctx)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if err := json.Unmarshal(b, &fc); err != nil {
			t.Fatalf("%s: bad JSON: %v", c.name, err)
		}
		if len(fc.Features) != c.count || fc.Features[0].Geometry.Type != c.geom {
			t.Fatalf("%s: got %d features of type %s, want %d %s", c.name,
				len(fc.Features), fc.Features[0].Geometry.Type, c.count, c.geom)
		}
	}
}

func TestNearestNode(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	for _, c := range []struct {
		name string
		p    geo.Point
		want string
	}{
		{"Missoula", missoula, "I-90"},
		{"Helena", helena, "I-15"},
		{"Glendive", glendive, "I-94"},
	} {
		got, err := s.NearestNode(ctx, c.p)
		if err != nil {
			t.Fatal(err)
		}
		if got.Highway != c.want || got.OffBy > 10 {
			t.Fatalf("%s snapped to %s, %.1f miles away, want %s within 10 miles", c.name, got.Highway, got.OffBy, c.want)
		}
	}
}

// Full trip from Helena to Glendive: snap, route over all three interstates,
// then look for points of interest along the way.
func TestRouteAcrossMontana(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	vertices, err := s.HighwayVertices(ctx)
	if err != nil {
		t.Fatal(err)
	}
	g := geo.NewGraph(vertices)

	from, _ := s.NearestNode(ctx, helena)
	to, _ := s.NearestNode(ctx, glendive)
	r, err := g.ShortestPath(from.Point, to.Point)
	if err != nil {
		t.Fatal(err)
	}
	hw := r.Highways()
	if len(hw) != 3 || hw[0] != "I-15" || hw[1] != "I-90" || hw[2] != "I-94" {
		t.Fatalf("Helena to Glendive used %v, want [I-15 I-90 I-94]", hw)
	}
	if r.Miles < 450 || r.Miles > 560 {
		t.Fatalf("Helena to Glendive is %.0f miles, expected 450-560", r.Miles)
	}

	line := make([]geo.Point, len(r.Steps))
	for i, st := range r.Steps {
		line[i] = st.Point
	}
	pois, err := s.POIsNearRoute(ctx, line, 5)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, p := range pois {
		names[p.Name] = true
	}
	for _, want := range []string{"Butte", "Bozeman", "Billings", "Miles City"} {
		if !names[want] {
			t.Errorf("expected %s along the route, got %v", want, names)
		}
	}
	if names["Missoula"] {
		t.Error("Missoula is not on the way from Helena to Glendive")
	}
}

func TestPasswordHash(t *testing.T) {
	s := testStore(t)
	if _, err := s.PasswordHash(context.Background(), "admin"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PasswordHash(context.Background(), "nobody"); err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
