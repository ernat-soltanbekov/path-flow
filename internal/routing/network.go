// Package routing finds minimum-turn schedules using minimum-cost flow.
package routing

import (
	"container/heap"
	"context"

	"github.com/ernat-soltanbekov/path-flow/internal/farm"
)

// Splitting a room into an entrance and an exit lets an ordinary flow
// algorithm enforce the rule "one ant per intermediate room".
type edge struct {
	to       int
	reverse  int
	capacity int
	cost     int
	tunnel   bool
}

type network struct {
	edges     [][]edge
	potential []int
	source    int
	sink      int
}

func newNetwork(f *farm.Farm) *network {
	n := &network{
		edges: make([][]edge, 2*len(f.Rooms)), potential: make([]int, 2*len(f.Rooms)),
		source: 2*f.Start + 1, sink: 2 * f.End,
	}
	for room := range f.Rooms {
		if room != f.Start && room != f.End {
			n.add(2*room, 2*room+1, 0, false)
		}
	}
	for _, t := range f.Tunnels {
		if t.A != f.End && t.B != f.Start {
			n.add(2*t.A+1, 2*t.B, 1, true)
		}
		if t.B != f.End && t.A != f.Start {
			n.add(2*t.B+1, 2*t.A, 1, true)
		}
	}
	return n
}

func (n *network) add(from, to, cost int, tunnel bool) {
	forward := edge{to: to, reverse: len(n.edges[to]), capacity: 1, cost: cost, tunnel: tunnel}
	backward := edge{to: from, reverse: len(n.edges[from]), cost: -cost}
	n.edges[from] = append(n.edges[from], forward)
	n.edges[to] = append(n.edges[to], backward)
}

const infinity = int(^uint(0)>>1) / 4

// augment finds one more unit of flow. Reverse edges allow it to undo an
// earlier route: greedily deleting the first shortest path would be wrong.
// Potentials turn negative reverse-edge costs into nonnegative reduced costs,
// so each search can use Dijkstra instead of repeated Bellman-Ford scans.
func (n *network) augment(ctx context.Context) (bool, error) {
	distance := make([]int, len(n.edges))
	previous := make([]int, len(n.edges))
	previousEdge := make([]int, len(n.edges))
	for v := range distance {
		distance[v] = infinity
	}
	distance[n.source] = 0
	queue := &priorityQueue{{node: n.source}}
	heap.Init(queue)
	for queue.Len() > 0 {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		item := heap.Pop(queue).(queueItem)
		v := item.node
		if item.distance != distance[v] {
			continue
		}
		for i, e := range n.edges[v] {
			if e.capacity == 0 {
				continue
			}
			candidate := distance[v] + e.cost + n.potential[v] - n.potential[e.to]
			if candidate < distance[e.to] {
				distance[e.to] = candidate
				previous[e.to], previousEdge[e.to] = v, i
				heap.Push(queue, queueItem{node: e.to, distance: candidate})
			}
		}
	}
	if distance[n.sink] == infinity {
		return false, nil
	}
	for v, d := range distance {
		if d < infinity {
			n.potential[v] += d
		}
	}
	for v := n.sink; v != n.source; v = previous[v] {
		from, index := previous[v], previousEdge[v]
		e := &n.edges[from][index]
		e.capacity--
		n.edges[v][e.reverse].capacity++
	}
	return true, nil
}

type queueItem struct{ node, distance int }
type priorityQueue []queueItem

func (q priorityQueue) Len() int { return len(q) }
func (q priorityQueue) Less(i, j int) bool {
	if q[i].distance == q[j].distance {
		return q[i].node < q[j].node
	}
	return q[i].distance < q[j].distance
}
func (q priorityQueue) Swap(i, j int) { q[i], q[j] = q[j], q[i] }
func (q *priorityQueue) Push(v any)   { *q = append(*q, v.(queueItem)) }
func (q *priorityQueue) Pop() any {
	last := len(*q) - 1
	v := (*q)[last]
	*q = (*q)[:last]
	return v
}
