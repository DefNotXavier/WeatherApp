package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/DefNotXavier/WeatherApp/server/geo"
	"github.com/DefNotXavier/WeatherApp/server/store"
	"github.com/DefNotXavier/WeatherApp/server/weather"
)

// Rough box around Montana with a little room on each side. Anything outside
// it gets rejected so the weather endpoint can't be used as a general NWS proxy.
func inMontana(p geo.Point) bool {
	return p.Lat >= 44.2 && p.Lat <= 49.2 && p.Lng >= -116.3 && p.Lng <= -103.8
}

type routeRequest struct {
	From geo.Point `json:"from"`
	To   geo.Point `json:"to"`
}

type routeResponse struct {
	From          store.Snapped    `json:"from"`
	To            store.Snapped    `json:"to"`
	DistanceMiles float64          `json:"distance_miles"`
	Highways      []string         `json:"highways"`
	Line          geoJSONLine      `json:"line"`
	Checkpoints   []geo.Checkpoint `json:"checkpoints"`
	Nearby        []store.POI      `json:"nearby"`
}

type geoJSONLine struct {
	Type        string       `json:"type"`
	Coordinates [][2]float64 `json:"coordinates"`
}

// planRoute snaps both ends to the highways, finds the shortest path, spaces
// out weather checkpoints, finds points of interest along the way, and logs
// the search.
func (s *Server) planRoute(w http.ResponseWriter, r *http.Request) {
	var req routeRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "expected JSON like {\"from\":{\"lat\":..,\"lng\":..},\"to\":{...}}")
		return
	}
	if !inMontana(req.From) || !inMontana(req.To) {
		writeError(w, http.StatusBadRequest, "both points need to be in Montana")
		return
	}

	ctx := r.Context()
	from, err := s.Store.NearestNode(ctx, req.From)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	to, err := s.Store.NearestNode(ctx, req.To)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	if from.Point == to.Point {
		writeError(w, http.StatusUnprocessableEntity, "start and end snap to the same spot, pick two places farther apart")
		return
	}

	route, err := s.Graph.ShortestPath(from.Point, to.Point)
	if errors.Is(err, geo.ErrNoRoute) {
		writeError(w, http.StatusUnprocessableEntity, "no route between those points")
		return
	}
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	line := make([]geo.Point, len(route.Steps))
	coords := make([][2]float64, len(route.Steps))
	for i, st := range route.Steps {
		line[i] = st.Point
		coords[i] = [2]float64{st.Lng, st.Lat} // GeoJSON is [lng, lat]
	}

	nearby, err := s.Store.POIsNearRoute(ctx, line, s.NearbyMiles)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	resp := routeResponse{
		From:          from,
		To:            to,
		DistanceMiles: route.Miles,
		Highways:      route.Highways(),
		Line:          geoJSONLine{Type: "LineString", Coordinates: coords},
		Checkpoints:   route.Checkpoints(s.CheckpointMiles),
		Nearby:        nearby,
	}
	if resp.Nearby == nil {
		resp.Nearby = []store.POI{}
	}

	// a failed log shouldn't break route planning for the user
	if err := s.Store.LogSearch(ctx, store.Search{
		Start:         from.Point,
		End:           to.Point,
		StartHighway:  from.Highway,
		EndHighway:    to.Highway,
		Highways:      resp.Highways,
		DistanceMiles: route.Miles,
		Checkpoints:   len(resp.Checkpoints),
	}); err != nil {
		s.Log.Error("logging search failed", "err", err)
	}

	writeJSON(w, http.StatusOK, resp)
}

// weather returns the next few forecast periods for a point. X-Cache says
// whether it came from Redis, which is handy for checking the cache works.
func (s *Server) weather(w http.ResponseWriter, r *http.Request) {
	lat, err1 := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
	lng, err2 := strconv.ParseFloat(r.URL.Query().Get("lng"), 64)
	if err1 != nil || err2 != nil {
		writeError(w, http.StatusBadRequest, "lat and lng are required")
		return
	}
	if !inMontana(geo.Point{Lat: lat, Lng: lng}) {
		writeError(w, http.StatusBadRequest, "point needs to be in Montana")
		return
	}

	periods, cached, err := s.Weather.Forecast(r.Context(), lat, lng)
	if errors.Is(err, weather.ErrNoForecast) {
		writeError(w, http.StatusNotFound, "no forecast for this location")
		return
	}
	if err != nil {
		s.Log.Warn("nws request failed", "lat", lat, "lng", lng, "err", err)
		writeError(w, http.StatusBadGateway, "could not reach the weather service")
		return
	}
	if cached {
		w.Header().Set("X-Cache", "HIT")
	} else {
		w.Header().Set("X-Cache", "MISS")
	}
	writeJSON(w, http.StatusOK, periods)
}
