package routing_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ernat-soltanbekov/path-flow/internal/farm"
	"github.com/ernat-soltanbekov/path-flow/internal/replay"
	"github.com/ernat-soltanbekov/path-flow/internal/routing"
)

func graph(t testing.TB, rooms, ants int, links [][2]int) *farm.Farm {
	t.Helper()
	var text strings.Builder
	fmt.Fprintln(&text, ants)
	for room := 0; room < rooms; room++ {
		if room == 0 {
			fmt.Fprintln(&text, "##start")
		}
		if room == rooms-1 {
			fmt.Fprintln(&text, "##end")
		}
		fmt.Fprintf(&text, "r%d %d %d\n", room, room, room%3)
	}
	for _, link := range links {
		fmt.Fprintf(&text, "r%d-r%d\n", link[0], link[1])
	}
	f, err := farm.Parse(strings.NewReader(text.String()))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func checkPlan(t testing.TB, f *farm.Farm, want int) {
	t.Helper()
	p, err := routing.Solve(context.Background(), f)
	if want < 0 {
		if err == nil {
			t.Fatal("accepted disconnected graph")
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if p.Turns != want {
		t.Fatalf("got %d turns, exact optimum %d\n%s", p.Turns, want, f.Source)
	}
	var output bytes.Buffer
	if err := routing.Write(context.Background(), &output, f, p); err != nil {
		t.Fatal(err)
	}
	r, err := replay.Parse(bytes.NewReader(output.Bytes()))
	if err != nil {
		t.Fatalf("invalid schedule: %v\n%s", err, output.String())
	}
	if len(r.Turns) != want {
		t.Fatalf("wrote %d turns, want %d", len(r.Turns), want)
	}
	// Also validate left-to-right vacancy, a stricter check than end-of-turn
	// occupancy: every emitted destination is already empty when it is used.
	occupied := make(map[int]int)
	for _, turn := range r.Turns {
		for _, move := range turn {
			delete(occupied, move.From)
			if move.To != f.End {
				if occupied[move.To] != 0 {
					t.Fatal("destination not yet vacated")
				}
				occupied[move.To] = move.Ant
			}
		}
	}
}

func TestKnownOptima(t *testing.T) {
	for _, tc := range []struct {
		name               string
		rooms, ants, turns int
		links              [][2]int
	}{
		{"direct tunnel is one ant per turn", 2, 10, 10, [][2]int{{0, 1}}},
		{"single pipeline", 4, 3, 5, [][2]int{{0, 1}, {1, 2}, {2, 3}}},
		{"parallel", 4, 3, 3, [][2]int{{0, 1}, {0, 2}, {1, 3}, {2, 3}, {1, 2}}},
		{"direct and detour", 3, 4, 3, [][2]int{{0, 2}, {0, 1}, {1, 2}}},
		{"no route", 4, 2, -1, [][2]int{{0, 1}, {2, 3}}},
		// Taking 0-1-2-7 first and deleting its rooms loses BOTH useful paths.
		{"reverse residual edges reroute", 8, 12, 9, [][2]int{{0, 1}, {1, 2}, {2, 7}, {1, 3}, {3, 4}, {4, 7}, {0, 5}, {5, 6}, {6, 2}}},
	} {
		t.Run(tc.name, func(t *testing.T) { checkPlan(t, graph(t, tc.rooms, tc.ants, tc.links), tc.turns) })
	}
}

// exactTurns knows nothing about flows or disjoint paths. It breadth-first
// searches ALL legal simultaneous moves, including waits and returning to
// start, and therefore acts as an independent small-graph optimality oracle.
func exactTurns(f *farm.Farm) int {
	encode := func(pos []int) string {
		ordered := append([]int(nil), pos...)
		sort.Ints(ordered) // Ants are interchangeable for finding a turn count.
		var key strings.Builder
		for _, room := range ordered {
			key.WriteByte(byte(room))
		}
		return key.String()
	}
	start := make([]int, f.Ants)
	for i := range start {
		start[i] = f.Start
	}
	seen := map[string]bool{encode(start): true}
	frontier := [][]int{start}
	for turns := 0; len(frontier) > 0; turns++ {
		var nextFrontier [][]int
		for _, pos := range frontier {
			complete := true
			for _, room := range pos {
				if room != f.End {
					complete = false
				}
			}
			if complete {
				return turns
			}
			next := make([]int, f.Ants)
			occupied := make([]bool, len(f.Rooms))
			used := make(map[[2]int]bool)
			var visit func(int)
			visit = func(ant int) {
				if ant == f.Ants {
					key := encode(next)
					if !seen[key] {
						seen[key] = true
						nextFrontier = append(nextFrontier, append([]int(nil), next...))
					}
					return
				}
				from := pos[ant]
				choices := []int{from}
				if from != f.End {
					choices = append(choices, f.Neighbors[from]...)
				}
				for _, to := range choices {
					limited := to != f.Start && to != f.End
					key := farm.TunnelKey(from, to)
					moving := from != to
					if (limited && occupied[to]) || (moving && used[key]) {
						continue
					}
					if limited {
						occupied[to] = true
					}
					if moving {
						used[key] = true
					}
					next[ant] = to
					visit(ant + 1)
					if limited {
						occupied[to] = false
					}
					if moving {
						delete(used, key)
					}
				}
			}
			visit(0)
		}
		frontier = nextFrontier
	}
	return -1
}

func TestExhaustiveSmallGraphs(t *testing.T) {
	for rooms := 2; rooms <= 5; rooms++ {
		var possible [][2]int
		for a := 0; a < rooms; a++ {
			for b := a + 1; b < rooms; b++ {
				possible = append(possible, [2]int{a, b})
			}
		}
		for mask := 0; mask < 1<<len(possible); mask++ {
			var links [][2]int
			for i, link := range possible {
				if mask&(1<<i) != 0 {
					links = append(links, link)
				}
			}
			for ants := 1; ants <= 3; ants++ {
				f := graph(t, rooms, ants, links)
				checkPlan(t, f, exactTurns(f))
			}
		}
	}
}

func TestRandomGraphsAgainstOracle(t *testing.T) {
	random := rand.New(rand.NewSource(2008))
	for trial := 0; trial < 250; trial++ {
		rooms := 6 + random.Intn(2)
		var links [][2]int
		for a := 0; a < rooms; a++ {
			for b := a + 1; b < rooms; b++ {
				if random.Intn(100) < 35 {
					links = append(links, [2]int{a, b})
				}
			}
		}
		f := graph(t, rooms, 1+random.Intn(4), links)
		checkPlan(t, f, exactTurns(f))
	}
}

func TestDeterminism(t *testing.T) {
	f := graph(t, 4, 25, [][2]int{{0, 1}, {0, 2}, {1, 3}, {2, 3}, {1, 2}})
	var previous string
	for run := 0; run < 10; run++ {
		p, err := routing.Solve(context.Background(), f)
		if err != nil {
			t.Fatal(err)
		}
		var b bytes.Buffer
		if err := routing.Write(context.Background(), &b, f, p); err != nil {
			t.Fatal(err)
		}
		if run > 0 && b.String() != previous {
			t.Fatal("nondeterministic output")
		}
		previous = b.String()
	}
}

func largeGraph(t testing.TB, ants int) *farm.Farm {
	const lanes, length = 40, 125
	end := lanes*(length-1) + 1
	var links [][2]int
	for lane := 0; lane < lanes; lane++ {
		previous := 0
		for step := 0; step < length-1; step++ {
			room := 1 + lane*(length-1) + step
			links = append(links, [2]int{previous, room})
			previous = room
		}
		links = append(links, [2]int{previous, end})
	}
	return graph(t, end+1, ants, links)
}

func TestLargeColonies(t *testing.T) {
	for _, ants := range []int{100, 1000} {
		t.Run(fmt.Sprint(ants), func(t *testing.T) {
			f := largeGraph(t, ants)
			start := time.Now()
			checkPlan(t, f, 125+(ants+39)/40-1)
			t.Logf("%d ants, %d rooms, %d tunnels: %s", ants, len(f.Rooms), len(f.Tunnels), time.Since(start))
		})
	}
}

type failedWriter struct{}

func (failedWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestFailures(t *testing.T) {
	f := graph(t, 2, 3, [][2]int{{0, 1}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := routing.Solve(ctx, f); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	if _, err := routing.Solve(context.Background(), nil); err == nil {
		t.Fatal("nil input")
	}
	p, err := routing.Solve(context.Background(), f)
	if err != nil {
		t.Fatal(err)
	}
	if err := routing.Write(context.Background(), failedWriter{}, f, p); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("write failure: %v", err)
	}
	if err := routing.Write(ctx, io.Discard, f, p); !errors.Is(err, context.Canceled) {
		t.Fatalf("write cancellation: %v", err)
	}
}

func BenchmarkLarge1000(b *testing.B) {
	f := largeGraph(b, 1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p, err := routing.Solve(context.Background(), f)
		if err != nil {
			b.Fatal(err)
		}
		if err := routing.Write(context.Background(), io.Discard, f, p); err != nil {
			b.Fatal(err)
		}
	}
}

func TestStressTopologies(t *testing.T) {
	t.Run("dense colony", func(t *testing.T) {
		var links [][2]int
		for a := 0; a < 120; a++ {
			for b := a + 1; b < 120; b++ {
				links = append(links, [2]int{a, b})
			}
		}
		// Direct route delivers T, 118 two-tunnel routes deliver T-1 each.
		checkPlan(t, graph(t, 120, 1000, links), 10)
	})
	t.Run("deep chain without recursion", func(t *testing.T) {
		var links [][2]int
		for room := 0; room < 14999; room++ {
			links = append(links, [2]int{room, room + 1})
		}
		checkPlan(t, graph(t, 15000, 3, links), 15001)
	})
	t.Run("many dead ends", func(t *testing.T) {
		var links [][2]int
		for room := 1; room < 10000; room++ {
			links = append(links, [2]int{0, room})
		}
		checkPlan(t, graph(t, 10001, 1000, links), -1)
	})
	t.Run("maximum ant count is streamed", func(t *testing.T) {
		f := graph(t, 2, farm.MaxAnts, [][2]int{{0, 1}})
		p, err := routing.Solve(context.Background(), f)
		if err != nil {
			t.Fatal(err)
		}
		if p.Turns != farm.MaxAnts {
			t.Fatal("direct tunnel capacity violated")
		}
		if err := routing.Write(context.Background(), io.Discard, f, p); err != nil {
			t.Fatal(err)
		}
	})
}
