package geo

// Checkpoint is a spot along a route where the app shows the weather.
type Checkpoint struct {
	Point
	Highway string  `json:"highway"`
	Mile    float64 `json:"mile"`
}

// Checkpoints spaces points out every `every` miles along the route, plus the
// start and the end. This replaces using every highway vertex, which were
// spaced unevenly and made a weather request for each one.
func (r Route) Checkpoints(every float64) []Checkpoint {
	if len(r.Steps) == 0 {
		return nil
	}
	first := r.Steps[0]
	out := []Checkpoint{{Point: first.Point, Highway: first.Highway}}
	if len(r.Steps) == 1 || every <= 0 {
		return out
	}

	next, traveled := every, 0.0
	for i := 1; i < len(r.Steps); i++ {
		a, b := r.Steps[i-1], r.Steps[i]
		seg := DistanceMiles(a.Point, b.Point)
		for seg > 0 && traveled+seg >= next {
			t := (next - traveled) / seg
			out = append(out, Checkpoint{Point: interpolate(a.Point, b.Point, t), Highway: b.Highway, Mile: next})
			next += every
		}
		traveled += seg
	}

	// Always end exactly at the destination. If the last checkpoint is right
	// before it, swap that one out instead of adding a near duplicate.
	last := r.Steps[len(r.Steps)-1]
	end := Checkpoint{Point: last.Point, Highway: last.Highway, Mile: traveled}
	if len(out) > 1 && traveled-out[len(out)-1].Mile < every/4 {
		out[len(out)-1] = end
	} else {
		out = append(out, end)
	}
	return out
}
