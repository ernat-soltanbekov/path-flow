// Package replay validates a solver transcript independently of its routing plan.
package replay

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"

	"github.com/ernat-soltanbekov/path-flow/internal/farm"
)

// The browser replay is bounded separately from the streaming solver.
const (
	MaxBytes = 64 << 20
	MaxMoves = 1_000_000
)

type Move struct {
	Ant  int `json:"ant"`
	From int `json:"from"`
	To   int `json:"to"`
}

type Replay struct {
	Farm  *farm.Farm `json:"farm"`
	Turns [][]Move   `json:"turns"`
}

// Parse checks every move, tunnel, occupancy, ant ID, and final arrival.
// A move may enter a room vacated during the same turn, in any token order.
func Parse(r io.Reader) (*Replay, error) {
	limited := &io.LimitedReader{R: r, N: MaxBytes + 1}
	scanner := bufio.NewScanner(limited)
	scanner.Buffer(make([]byte, 4096), 32<<20)
	var header strings.Builder
	var result *Replay
	var v *validator
	moveCount, lineNumber := 0, 0
	trailingBlank := false
	for scanner.Scan() {
		lineNumber++
		raw := scanner.Text()
		line := strings.TrimSpace(raw)
		if result == nil {
			if !strings.HasPrefix(line, "L") {
				if header.Len()+len(raw)+1 > farm.MaxInputBytes {
					return nil, fmt.Errorf("colony header too large")
				}
				header.WriteString(raw)
				header.WriteByte('\n')
				continue
			}
			f, err := farm.Parse(strings.NewReader(header.String()))
			if err != nil {
				return nil, fmt.Errorf("invalid colony header: %w", err)
			}
			result = &Replay{Farm: f}
			v = newValidator(f)
		}
		if line == "" {
			trailingBlank = true
			continue
		}
		if trailingBlank {
			return nil, fmt.Errorf("empty movement turn before line %d", lineNumber)
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		// An internal room can provide at most one departing ant. Start can send
		// at most one ant to each other room. This also bounds hostile token lists
		// BEFORE allocating a slice or the per-turn validation maps.
		maximum := min(MaxMoves-moveCount, min(result.Farm.Ants, 2*len(result.Farm.Rooms)-3))
		tokens, err := movementTokens(line, maximum)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNumber, err)
		}
		moveCount += len(tokens)
		moves, err := v.turn(tokens)
		if err != nil {
			return nil, fmt.Errorf("turn %d: %w", len(result.Turns)+1, err)
		}
		result.Turns = append(result.Turns, moves)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read transcript (maximum line 32 MiB): %w", err)
	}
	if limited.N == 0 {
		return nil, fmt.Errorf("transcript exceeds %d bytes", MaxBytes)
	}
	if result == nil {
		return nil, fmt.Errorf("transcript contains no ant movements")
	}
	if v.occupancy[result.Farm.End] != result.Farm.Ants {
		return nil, fmt.Errorf("only %d of %d ants reached ##end", v.occupancy[result.Farm.End], result.Farm.Ants)
	}
	return result, nil
}

func movementTokens(line string, maximum int) ([]string, error) {
	count, inToken := 0, false
	for _, r := range line {
		if unicode.IsSpace(r) {
			inToken = false
			continue
		}
		if !inToken {
			count++
			if count > maximum {
				return nil, fmt.Errorf("too many movements for this turn or the %d-move browser limit", MaxMoves)
			}
			inToken = true
		}
	}
	return strings.Fields(line), nil
}

type validator struct {
	farm      *farm.Farm
	names     map[string]int
	links     map[[2]int]bool
	positions []int
	occupancy []int
}

func newValidator(f *farm.Farm) *validator {
	v := &validator{
		farm: f, names: make(map[string]int), links: make(map[[2]int]bool),
		positions: make([]int, f.Ants+1), occupancy: make([]int, len(f.Rooms)),
	}
	for i, room := range f.Rooms {
		v.names[room.Name] = i
	}
	for _, t := range f.Tunnels {
		v.links[farm.TunnelKey(t.A, t.B)] = true
	}
	for ant := 1; ant <= f.Ants; ant++ {
		v.positions[ant] = f.Start
	}
	v.occupancy[f.Start] = f.Ants
	return v
}

func (v *validator) turn(tokens []string) ([]Move, error) {
	usedAnts := make(map[int]bool, len(tokens))
	usedTunnels := make(map[[2]int]bool, len(tokens))
	touched := make(map[int]bool, 2*len(tokens))
	moves := make([]Move, 0, len(tokens))
	for _, token := range tokens {
		antText, roomName, ok := strings.Cut(token, "-")
		if !ok || len(antText) < 2 || antText[0] != 'L' {
			return nil, fmt.Errorf("invalid movement %q", token)
		}
		ant, err := strconv.Atoi(antText[1:])
		if err != nil || ant < 1 || ant > v.farm.Ants || strconv.Itoa(ant) != antText[1:] {
			return nil, fmt.Errorf("invalid ant number %q", antText)
		}
		if usedAnts[ant] {
			return nil, fmt.Errorf("ant %d moves more than once", ant)
		}
		to, exists := v.names[roomName]
		if !exists {
			return nil, fmt.Errorf("unknown destination %q", roomName)
		}
		from := v.positions[ant]
		if from == v.farm.End {
			return nil, fmt.Errorf("ant %d moves after reaching ##end", ant)
		}
		key := farm.TunnelKey(from, to)
		if !v.links[key] {
			return nil, fmt.Errorf("ant %d uses a nonexistent tunnel", ant)
		}
		if usedTunnels[key] {
			return nil, fmt.Errorf("tunnel used more than once")
		}
		usedAnts[ant], usedTunnels[key] = true, true
		touched[from], touched[to] = true, true
		moves = append(moves, Move{Ant: ant, From: from, To: to})
	}
	for _, move := range moves {
		v.positions[move.Ant] = move.To
		v.occupancy[move.From]--
		v.occupancy[move.To]++
	}
	for room := range touched {
		if room != v.farm.Start && room != v.farm.End && v.occupancy[room] > 1 {
			return nil, fmt.Errorf("collision in room %q", v.farm.Rooms[room].Name)
		}
	}
	return moves, nil
}
