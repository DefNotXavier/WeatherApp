// Package geo has the routing logic: distances, the highway graph, Dijkstra's
// shortest path, and spacing weather checkpoints out along a route.
package geo

import "math"

const earthRadiusMiles = 3958.8

// Point is a WGS 84 coordinate.
type Point struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// DistanceMiles is the great-circle distance between two points (haversine).
func DistanceMiles(a, b Point) float64 {
	toRad := func(d float64) float64 { return d * math.Pi / 180 }
	dLat := toRad(b.Lat - a.Lat)
	dLng := toRad(b.Lng - a.Lng)
	h := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(toRad(a.Lat))*math.Cos(toRad(b.Lat))*math.Sin(dLng/2)*math.Sin(dLng/2)
	return earthRadiusMiles * 2 * math.Asin(math.Sqrt(h))
}

// interpolate returns the point t (0..1) of the way from a to b. Highway
// vertices are only a few miles apart, so a straight line between them is
// close enough.
func interpolate(a, b Point, t float64) Point {
	return Point{Lat: a.Lat + (b.Lat-a.Lat)*t, Lng: a.Lng + (b.Lng-a.Lng)*t}
}
