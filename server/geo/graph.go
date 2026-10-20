package geo

import (
	"container/heap"
	"errors"
	"sort"
	"strconv"
)

var (
	ErrUnknownPoint = errors.New("point is not a highway node")
	ErrNoRoute      = errors.New("no route between those points")
)

// Vertex is one point on a highway line. Seq is its position along the line.
type Vertex struct {
	Highway string
	Seq     int
	Point
}

// Graph is the highway network. Every vertex is a node, and vertices next to
// each other on a highway are connected by an edge weighted in miles. A
// junction is a vertex that shows up on two highways with the same
// coordinates, so it becomes one node that links them.
type Graph struct {
	nodes map[string]*node
}

type node struct {
	pt       Point
	highways []string
	edges    []edge
}

type edge struct {
	to      string
	miles   float64
	highway string
}

// Step is one node along a route, with the highway used to get there.
type Step struct {
	Point
	Highway string `json:"highway"`
}

// Route is the result of a shortest path search.
type Route struct {
	Steps []Step
	Miles float64
}

// key identifies a node by its coordinates. Six decimals is about 10 cm.
func key(p Point) string {
	return strconv.FormatFloat(p.Lat, 'f', 6, 64) + "," + strconv.FormatFloat(p.Lng, 'f', 6, 64)
}

// NewGraph builds the graph from highway vertices.
func NewGraph(vertices []Vertex) *Graph {
	g := &Graph{nodes: map[string]*node{}}

	byHighway := map[string][]Vertex{}
	for _, v := range vertices {
		byHighway[v.Highway] = append(byHighway[v.Highway], v)
	}

	for hw, vs := range byHighway {
		sort.Slice(vs, func(i, j int) bool { return vs[i].Seq < vs[j].Seq })
		for i, v := range vs {
			k := key(v.Point)
			n, ok := g.nodes[k]
			if !ok {
				n = &node{pt: v.Point}
				g.nodes[k] = n
			}
			n.highways = append(n.highways, hw)
			if i == 0 {
				continue
			}
			prev := key(vs[i-1].Point)
			miles := DistanceMiles(vs[i-1].Point, v.Point)
			g.nodes[prev].edges = append(g.nodes[prev].edges, edge{to: k, miles: miles, highway: hw})
			n.edges = append(n.edges, edge{to: prev, miles: miles, highway: hw})
		}
	}
	return g
}

// Len is the number of nodes in the graph.
func (g *Graph) Len() int { return len(g.nodes) }

// ShortestPath runs Dijkstra's algorithm between two nodes. Both points need
// to be exact highway vertices, so snap them first.
func (g *Graph) ShortestPath(from, to Point) (Route, error) {
	start, end := key(from), key(to)
	if g.nodes[start] == nil || g.nodes[end] == nil {
		return Route{}, ErrUnknownPoint
	}
	if start == end {
		n := g.nodes[start]
		return Route{Steps: []Step{{Point: n.pt, Highway: n.highways[0]}}}, nil
	}

	type prevStep struct {
		from    string
		highway string
	}
	dist := map[string]float64{start: 0}
	prev := map[string]prevStep{}
	done := map[string]bool{}
	pq := &queue{{key: start}}

	for pq.Len() > 0 {
		cur := heap.Pop(pq).(item)
		if done[cur.key] {
			continue
		}
		done[cur.key] = true
		if cur.key == end {
			break
		}
		for _, e := range g.nodes[cur.key].edges {
			d := dist[cur.key] + e.miles
			if old, seen := dist[e.to]; !seen || d < old {
				dist[e.to] = d
				prev[e.to] = prevStep{from: cur.key, highway: e.highway}
				heap.Push(pq, item{key: e.to, miles: d})
			}
		}
	}
	if !done[end] {
		return Route{}, ErrNoRoute
	}

	// Walk back from the end. Each node takes the highway of the edge we
	// drove in on, and the start takes the highway of the first edge.
	var steps []Step
	for k := end; k != start; k = prev[k].from {
		steps = append(steps, Step{Point: g.nodes[k].pt, Highway: prev[k].highway})
	}
	steps = append(steps, Step{Point: g.nodes[start].pt, Highway: steps[len(steps)-1].Highway})
	for i, j := 0, len(steps)-1; i < j; i, j = i+1, j-1 {
		steps[i], steps[j] = steps[j], steps[i]
	}
	return Route{Steps: steps, Miles: dist[end]}, nil
}

// Highways lists the highways a route uses, in the order you drive them.
func (r Route) Highways() []string {
	var out []string
	for _, s := range r.Steps {
		if len(out) == 0 || out[len(out)-1] != s.Highway {
			out = append(out, s.Highway)
		}
	}
	return out
}

// priority queue for Dijkstra, closest node first
type item struct {
	key   string
	miles float64
}

type queue []item

func (q queue) Len() int           { return len(q) }
func (q queue) Less(i, j int) bool { return q[i].miles < q[j].miles }
func (q queue) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q *queue) Push(x any)        { *q = append(*q, x.(item)) }
func (q *queue) Pop() any {
	old := *q
	it := old[len(old)-1]
	*q = old[:len(old)-1]
	return it
}
