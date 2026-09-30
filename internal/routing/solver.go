package routing

import (
	"context"
	"fmt"
	"sort"

	"github.com/ernat-soltanbekov/path-flow/internal/farm"
)

// Plan stores pipelines, not a turn-by-turn copy of every ant's journey.
// Paths include start and end; Counts says how many ants enter each path.
type Plan struct {
	Paths  [][]int
	Counts []int
	Turns  int
}

// Solve expects a farm returned by farm.Parse. Every intermediate room and
// every tunnel has unit capacity. See docs/ALGORITHM.md for the optimality
// argument and the independent exhaustive tests in solver_test.go.
func Solve(ctx context.Context, f *farm.Farm) (*Plan, error) {
	if f == nil || f.Ants < 1 || f.Start < 0 || f.End < 0 || f.Start == f.End ||
		f.Start >= len(f.Rooms) || f.End >= len(f.Rooms) {
		return nil, fmt.Errorf("invalid colony endpoints or ant count")
	}
	n := newNetwork(f)
	var best *Plan
	for count := 1; count <= f.Ants; count++ {
		found, err := n.augment(ctx)
		if err != nil {
			return nil, err
		}
		if !found {
			break
		}
		paths, err := n.paths(f)
		if err != nil {
			return nil, err
		}
		turns := completionTime(paths, f.Ants)
		if best == nil || turns < best.Turns {
			best = allocate(paths, f.Ants, turns)
		}
		// At horizon best.Turns-1, a new flow unit contributes
		// best.Turns - marginalCost arrivals. Marginal costs cannot decrease.
		// A nonpositive contribution means no later flow can beat this plan.
		if n.potential[n.sink]-n.potential[n.source] >= best.Turns {
			break
		}
	}
	if best == nil {
		return nil, fmt.Errorf("no path from ##start to ##end")
	}
	return best, nil
}

func (n *network) paths(f *farm.Farm) ([][]int, error) {
	var paths [][]int
	for _, first := range n.edges[n.source] {
		if !first.tunnel || first.capacity != 0 {
			continue
		}
		path := []int{f.Start, first.to / 2}
		for path[len(path)-1] != f.End {
			current := path[len(path)-1]
			next := -1
			for _, e := range n.edges[2*current+1] {
				if e.tunnel && e.capacity == 0 {
					next = e.to / 2
					break
				}
			}
			if next < 0 || len(path) > len(f.Rooms) {
				return nil, fmt.Errorf("cannot decompose routing flow")
			}
			path = append(path, next)
		}
		paths = append(paths, path)
	}
	sort.Slice(paths, func(i, j int) bool {
		if len(paths[i]) != len(paths[j]) {
			return len(paths[i]) < len(paths[j])
		}
		for k := range paths[i] {
			left, right := f.Rooms[paths[i][k]].Name, f.Rooms[paths[j][k]].Name
			if left != right {
				return left < right
			}
		}
		return false
	})
	return paths, nil
}

// A path of d tunnels delivers max(0, turns-d+1) ants. Binary search
// finds the first horizon whose total capacity fits the whole colony.
func completionTime(paths [][]int, ants int) int {
	low, high := len(paths[0])-1, len(paths[0])-2+ants
	for low < high {
		middle := low + (high-low)/2
		capacity := 0
		for _, path := range paths {
			capacity += max(0, middle-len(path)+2)
			if capacity >= ants {
				break
			}
		}
		if capacity >= ants {
			high = middle
		} else {
			low = middle + 1
		}
	}
	return low
}

func allocate(paths [][]int, ants, turns int) *Plan {
	plan := &Plan{Turns: turns}
	for _, path := range paths {
		count := min(ants, max(0, turns-len(path)+2))
		if count > 0 {
			plan.Paths = append(plan.Paths, path)
			plan.Counts = append(plan.Counts, count)
			ants -= count
		}
	}
	return plan
}
