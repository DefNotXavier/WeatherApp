// Package store has all the PostGIS queries.
package store

import (
	"context"
	"errors"
	"time"

	"github.com/DefNotXavier/WeatherApp/server/geo"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const metersPerMile = 1609.344

var ErrNotFound = errors.New("not found")

type Store struct {
	DB *pgxpool.Pool
}

func Open(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	return &Store{DB: pool}, nil
}

func (s *Store) Close()                         { s.DB.Close() }
func (s *Store) Ping(ctx context.Context) error { return s.DB.Ping(ctx) }

// HighwaysGeoJSON returns every highway line as a GeoJSON FeatureCollection.
// PostGIS builds the JSON, so it gets passed straight through to the browser.
func (s *Store) HighwaysGeoJSON(ctx context.Context) ([]byte, error) {
	return s.featureCollection(ctx, `
		SELECT json_build_object(
			'type', 'Feature',
			'geometry', ST_AsGeoJSON(geom)::json,
			'properties', json_build_object('name', name)
		) AS feature, name AS sort
		FROM highways`)
}

func (s *Store) BorderGeoJSON(ctx context.Context) ([]byte, error) {
	return s.featureCollection(ctx, `
		SELECT json_build_object(
			'type', 'Feature',
			'geometry', ST_AsGeoJSON(geom)::json,
			'properties', json_build_object('name', name)
		) AS feature, name AS sort
		FROM border`)
}

func (s *Store) POIsGeoJSON(ctx context.Context) ([]byte, error) {
	return s.featureCollection(ctx, `
		SELECT json_build_object(
			'type', 'Feature',
			'geometry', ST_AsGeoJSON(geom)::json,
			'properties', json_build_object('id', id, 'name', name, 'type', type)
		) AS feature, type || name AS sort
		FROM pois`)
}

func (s *Store) featureCollection(ctx context.Context, features string) ([]byte, error) {
	var out []byte
	err := s.DB.QueryRow(ctx, `
		SELECT json_build_object(
			'type', 'FeatureCollection',
			'features', COALESCE(json_agg(f.feature ORDER BY f.sort), '[]'::json)
		)
		FROM (`+features+`) f`).Scan(&out)
	return out, err
}

// HighwayVertices loads every routing node for building the graph.
func (s *Store) HighwayVertices(ctx context.Context) ([]geo.Vertex, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT highway, seq, ST_Y(geom), ST_X(geom)
		FROM highway_nodes
		ORDER BY highway, seq`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (geo.Vertex, error) {
		var v geo.Vertex
		err := r.Scan(&v.Highway, &v.Seq, &v.Lat, &v.Lng)
		return v, err
	})
}

// Snapped is a clicked location moved onto the nearest highway node.
type Snapped struct {
	geo.Point
	Highway string  `json:"highway"`
	OffBy   float64 `json:"off_by_miles"`
}

// NearestNode snaps a point to the closest highway node. The <-> operator is
// a KNN search that uses the GIST index on highway_nodes.
func (s *Store) NearestNode(ctx context.Context, p geo.Point) (Snapped, error) {
	var out Snapped
	err := s.DB.QueryRow(ctx, `
		SELECT highway, ST_Y(geom), ST_X(geom),
		       ST_Distance(geom::geography, ST_SetSRID(ST_MakePoint($2, $1), 4326)::geography) / $3
		FROM highway_nodes
		ORDER BY geom <-> ST_SetSRID(ST_MakePoint($2, $1), 4326)
		LIMIT 1`, p.Lat, p.Lng, metersPerMile).Scan(&out.Highway, &out.Lat, &out.Lng, &out.OffBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrNotFound
	}
	return out, err
}

// POI is a point of interest near a route.
type POI struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	geo.Point
	OffRouteMiles float64 `json:"off_route_miles"`
}

// POIsNearRoute finds points of interest within `miles` of a route line,
// ordered by how far along the route they are. Casting to geography makes
// ST_DWithin measure in real meters instead of degrees.
func (s *Store) POIsNearRoute(ctx context.Context, route []geo.Point, miles float64) ([]POI, error) {
	if len(route) < 2 {
		return nil, nil
	}
	lngs := make([]float64, len(route))
	lats := make([]float64, len(route))
	for i, p := range route {
		lngs[i], lats[i] = p.Lng, p.Lat
	}
	rows, err := s.DB.Query(ctx, `
		WITH r AS (
			SELECT ST_SetSRID(ST_MakeLine(ST_MakePoint(x, y) ORDER BY n), 4326) AS line
			FROM unnest($1::float8[], $2::float8[]) WITH ORDINALITY AS t(x, y, n)
		)
		SELECT p.id, p.name, p.type, ST_Y(p.geom), ST_X(p.geom),
		       ST_Distance(p.geom::geography, r.line::geography) / $4
		FROM pois p, r
		WHERE ST_DWithin(p.geom::geography, r.line::geography, $3)
		ORDER BY ST_LineLocatePoint(r.line, p.geom)`,
		lngs, lats, miles*metersPerMile, metersPerMile)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (POI, error) {
		var p POI
		err := r.Scan(&p.ID, &p.Name, &p.Type, &p.Lat, &p.Lng, &p.OffRouteMiles)
		return p, err
	})
}

// Search is one row of the route search log.
type Search struct {
	Start, End    geo.Point
	StartHighway  string
	EndHighway    string
	Highways      []string
	DistanceMiles float64
	Checkpoints   int
	SearchedAt    time.Time
}

func (s *Store) LogSearch(ctx context.Context, x Search) error {
	_, err := s.DB.Exec(ctx, `
		INSERT INTO search_log
			(start_geom, end_geom, start_highway, end_highway, highways, distance_miles, checkpoints)
		VALUES
			(ST_SetSRID(ST_MakePoint($1, $2), 4326), ST_SetSRID(ST_MakePoint($3, $4), 4326),
			 $5, $6, $7, $8, $9)`,
		x.Start.Lng, x.Start.Lat, x.End.Lng, x.End.Lat,
		x.StartHighway, x.EndHighway, x.Highways, x.DistanceMiles, x.Checkpoints)
	return err
}

// RecentSearches returns the newest searches first.
func (s *Store) RecentSearches(ctx context.Context, limit int) ([]Search, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT ST_Y(start_geom), ST_X(start_geom), ST_Y(end_geom), ST_X(end_geom),
		       start_highway, end_highway, highways, distance_miles::float8, checkpoints, searched_at
		FROM search_log
		ORDER BY searched_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Search, error) {
		var x Search
		err := r.Scan(&x.Start.Lat, &x.Start.Lng, &x.End.Lat, &x.End.Lng,
			&x.StartHighway, &x.EndHighway, &x.Highways, &x.DistanceMiles, &x.Checkpoints, &x.SearchedAt)
		return x, err
	})
}

// PasswordHash returns the bcrypt hash for a user.
func (s *Store) PasswordHash(ctx context.Context, username string) (string, error) {
	var hash string
	err := s.DB.QueryRow(ctx, `SELECT password_hash FROM users WHERE username = $1`, username).Scan(&hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return hash, err
}
