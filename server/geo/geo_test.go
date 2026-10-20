package geo

import (
	"errors"
	"math"
	"reflect"
	"testing"
)

func TestDistanceMiles(t *testing.T) {
	// one degree of latitude is about 69 miles
	got := DistanceMiles(Point{45, -110}, Point{46, -110})
	if math.Abs(got-69.1) > 0.1 {
		t.Fatalf("1 degree of latitude = %.2f miles, want about 69.1", got)
	}
	// Missoula to Billings is about 290 miles in a straight line
	got = DistanceMiles(Point{46.8721, -113.9940}, Point{45.7833, -108.5007})
	if got < 270 || got > 300 {
		t.Fatalf("Missoula to Billings = %.0f miles, want 270-300", got)
	}
}

// Three highways shaped like the real ones: A runs west to east, B goes north
// from A's middle, and C goes east from A's end.
//
//	        B2
//	        |
//	A1 --- A2/B1 --- A3/C1 --- C2
func testGraph() *Graph {
	return NewGraph([]Vertex{
		{"A", 1, Point{45, -112}},
		{"A", 2, Point{45, -111}},
		{"A", 3, Point{45, -110}},
		{"B", 1, Point{45, -111}},
		{"B", 2, Point{46, -111}},
		// out of order on purpose, NewGraph should sort by Seq
		{"C", 2, Point{45, -109}},
		{"C", 1, Point{45, -110}},
	})
}

func TestNewGraphMergesJunctions(t *testing.T) {
	// 7 vertices, but 2 of them are junctions shared by two highways
	if got := testGraph().Len(); got != 5 {
		t.Fatalf("graph has %d nodes, want 5", got)
	}
}

func TestShortestPathAcrossJunctions(t *testing.T) {
	r, err := testGraph().ShortestPath(Point{46, -111}, Point{45, -109})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"B", "A", "C"}
	if got := r.Highways(); !reflect.DeepEqual(got, want) {
		t.Fatalf("highways = %v, want %v", got, want)
	}
	if len(r.Steps) != 4 {
		t.Fatalf("route has %d steps, want 4", len(r.Steps))
	}
	// the junction we turn at belongs to the highway we drove in on
	if r.Steps[1].Highway != "B" {
		t.Fatalf("junction step highway = %s, want B", r.Steps[1].Highway)
	}
}

func TestShortestPathPicksShorterRoad(t *testing.T) {
	// Two ways from X to Y: a long way around on "Long", or a direct road
	g := NewGraph([]Vertex{
		{"Long", 1, Point{45, -112}},
		{"Long", 2, Point{47, -111}},
		{"Long", 3, Point{45, -110}},
		{"Short", 1, Point{45, -112}},
		{"Short", 2, Point{45, -110}},
	})
	r, err := g.ShortestPath(Point{45, -112}, Point{45, -110})
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Highways(); !reflect.DeepEqual(got, []string{"Short"}) {
		t.Fatalf("highways = %v, want [Short]", got)
	}
}

func TestShortestPathSamePoint(t *testing.T) {
	r, err := testGraph().ShortestPath(Point{45, -111}, Point{45, -111})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Steps) != 1 || r.Miles != 0 {
		t.Fatalf("got %d steps and %.1f miles, want 1 step and 0 miles", len(r.Steps), r.Miles)
	}
}

func TestShortestPathErrors(t *testing.T) {
	g := NewGraph([]Vertex{
		{"A", 1, Point{45, -112}},
		{"A", 2, Point{45, -111}},
		{"Island", 1, Point{48, -105}},
		{"Island", 2, Point{48, -104}},
	})
	if _, err := g.ShortestPath(Point{45, -112}, Point{48, -104}); !errors.Is(err, ErrNoRoute) {
		t.Fatalf("disconnected highways: err = %v, want ErrNoRoute", err)
	}
	if _, err := g.ShortestPath(Point{45, -112}, Point{1, 1}); !errors.Is(err, ErrUnknownPoint) {
		t.Fatalf("unknown point: err = %v, want ErrUnknownPoint", err)
	}
}

func TestCheckpoints(t *testing.T) {
	// A straight road about 207 miles long (3 degrees of latitude)
	g := NewGraph([]Vertex{
		{"N", 1, Point{45, -110}},
		{"N", 2, Point{46, -110}},
		{"N", 3, Point{47, -110}},
		{"N", 4, Point{48, -110}},
	})
	r, err := g.ShortestPath(Point{45, -110}, Point{48, -110})
	if err != nil {
		t.Fatal(err)
	}
	cps := r.Checkpoints(15)

	// start + one every 15 miles (13 of them) + end
	if len(cps) != 15 {
		t.Fatalf("got %d checkpoints, want 15", len(cps))
	}
	if cps[0].Point != (Point{45, -110}) || cps[len(cps)-1].Point != (Point{48, -110}) {
		t.Fatal("checkpoints should start and end at the route ends")
	}
	for i := 1; i < len(cps)-1; i++ {
		gap := DistanceMiles(cps[i-1].Point, cps[i].Point)
		if math.Abs(gap-15) > 0.01 {
			t.Fatalf("checkpoint %d is %.2f miles from the last one, want 15", i, gap)
		}
	}
}

func TestCheckpointsNoNearDuplicateAtEnd(t *testing.T) {
	// 30.3 miles: a checkpoint lands at mile 30, which is basically the end
	g := NewGraph([]Vertex{{"N", 1, Point{45, -110}}, {"N", 2, Point{45.4385, -110}}})
	r, _ := g.ShortestPath(Point{45, -110}, Point{45.4385, -110})
	cps := r.Checkpoints(15)
	if len(cps) != 3 {
		t.Fatalf("got %d checkpoints, want 3 (start, mile 15, end)", len(cps))
	}
}
