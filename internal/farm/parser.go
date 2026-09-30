package farm

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Parse validates the complete file before returning a graph. Comments and
// unknown commands are retained for output but never interpreted as rooms.
func Parse(r io.Reader) (*Farm, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxInputBytes+1))
	if err != nil {
		return nil, fmt.Errorf("cannot read colony: %w", err)
	}
	if len(data) > MaxInputBytes {
		return nil, fmt.Errorf("file exceeds %d bytes", MaxInputBytes)
	}
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return nil, fmt.Errorf("input must be UTF-8 text without NUL bytes")
	}
	source := strings.TrimPrefix(strings.ReplaceAll(string(data), "\r\n", "\n"), "\uFEFF")
	p := parser{
		farm:  &Farm{Start: -1, End: -1, Source: strings.TrimRight(source, "\n") + "\n"},
		names: make(map[string]int), links: make(map[[2]int]bool),
	}
	scanner := bufio.NewScanner(strings.NewReader(source))
	scanner.Buffer(make([]byte, 1024), MaxLineBytes+1)
	for scanner.Scan() {
		p.line++
		if len(scanner.Text()) > MaxLineBytes {
			return nil, p.fail("line exceeds %d bytes", MaxLineBytes)
		}
		if err := p.readLine(scanner.Text()); err != nil {
			return nil, err
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, p.fail("line too long or unreadable: %v", err)
	}
	if p.farm.Ants == 0 {
		return nil, fmt.Errorf("missing ant count")
	}
	if p.pending != "" {
		return nil, fmt.Errorf("%s has no room", p.pending)
	}
	if p.farm.Start < 0 || p.farm.End < 0 {
		return nil, fmt.Errorf("exactly one ##start and one ##end room are required")
	}
	p.farm.Neighbors = make([][]int, len(p.farm.Rooms))
	for _, link := range p.farm.Tunnels {
		p.farm.Neighbors[link.A] = append(p.farm.Neighbors[link.A], link.B)
		p.farm.Neighbors[link.B] = append(p.farm.Neighbors[link.B], link.A)
	}
	return p.farm, nil
}

type parser struct {
	farm    *Farm
	names   map[string]int
	links   map[[2]int]bool
	pending string
	inLinks bool
	line    int
}

func (p *parser) fail(format string, args ...any) error {
	return fmt.Errorf("line %d: %s", p.line, fmt.Sprintf(format, args...))
}

func (p *parser) readLine(line string) error {
	if strings.TrimSpace(line) == "" {
		return nil
	}
	if line == "##start" || line == "##end" {
		if p.farm.Ants == 0 || p.inLinks || p.pending != "" {
			return p.fail("misplaced %s command", line)
		}
		if (line == "##start" && p.farm.Start >= 0) || (line == "##end" && p.farm.End >= 0) {
			return p.fail("duplicate %s command", line)
		}
		p.pending = line
		return nil
	}
	if strings.HasPrefix(line, "#") {
		return nil
	}
	if p.farm.Ants == 0 {
		for _, c := range line {
			if c < '0' || c > '9' {
				return p.fail("ant count must be a positive decimal integer")
			}
		}
		ants, err := strconv.Atoi(line)
		if err != nil || ants < 1 || ants > MaxAnts {
			return p.fail("ant count must be between 1 and %d", MaxAnts)
		}
		p.farm.Ants = ants
		return nil
	}
	fields := strings.Fields(line)
	if len(fields) == 3 {
		return p.room(fields)
	}
	if len(fields) != 1 || strings.TrimSpace(line) != line {
		return p.fail("expected 'name x y' or 'name-name'")
	}
	return p.tunnel(line)
}

func (p *parser) room(fields []string) error {
	if p.inLinks {
		return p.fail("room declarations must precede tunnels")
	}
	name := fields[0]
	if len(name) > MaxNameBytes || strings.HasPrefix(name, "L") || strings.HasPrefix(name, "#") {
		return p.fail("invalid room name %q", name)
	}
	for _, c := range name {
		if unicode.IsControl(c) || unicode.IsSpace(c) {
			return p.fail("invalid character in room name")
		}
	}
	if _, exists := p.names[name]; exists {
		return p.fail("duplicate room %q", name)
	}
	x, xerr := strconv.Atoi(fields[1])
	y, yerr := strconv.Atoi(fields[2])
	if xerr != nil || yerr != nil {
		return p.fail("coordinates of %q must be integers in this platform's int range", name)
	}
	if len(p.farm.Rooms) >= MaxRooms {
		return p.fail("more than %d rooms", MaxRooms)
	}
	id := len(p.farm.Rooms)
	p.names[name] = id
	p.farm.Rooms = append(p.farm.Rooms, Room{Name: name, X: x, Y: y})
	switch p.pending {
	case "##start":
		p.farm.Start = id
	case "##end":
		p.farm.End = id
	}
	p.pending = ""
	return nil
}

func (p *parser) tunnel(line string) error {
	if p.pending != "" {
		return p.fail("%s must be followed by a room", p.pending)
	}
	// A hyphen is legal in a room name. Resolve the separator against declared
	// names, and reject genuinely ambiguous links instead of guessing.
	var a, b, matches int
	for i := 1; i < len(line)-1; i++ {
		if line[i] != '-' {
			continue
		}
		left, lok := p.names[line[:i]]
		right, rok := p.names[line[i+1:]]
		if lok && rok {
			a, b = left, right
			matches++
		}
	}
	if matches != 1 {
		return p.fail("tunnel %q has unknown rooms or an ambiguous separator", line)
	}
	if a == b {
		return p.fail("self-link on room %q", p.farm.Rooms[a].Name)
	}
	key := TunnelKey(a, b)
	if p.links[key] {
		return p.fail("duplicate tunnel %q", line)
	}
	if len(p.farm.Tunnels) >= MaxTunnels {
		return p.fail("more than %d tunnels", MaxTunnels)
	}
	p.links[key] = true
	p.farm.Tunnels = append(p.farm.Tunnels, Tunnel{A: a, B: b})
	p.inLinks = true
	return nil
}
