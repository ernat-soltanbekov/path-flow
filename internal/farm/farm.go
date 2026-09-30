// Package farm parses the input language into a simple undirected graph.
package farm

// Resource limits reject unreasonable inputs before they exhaust memory.
// They are intentionally much larger than the audit's 100/1000-ant examples.
const (
	MaxInputBytes = 32 << 20
	MaxLineBytes  = 4096
	MaxNameBytes  = 256
	MaxAnts       = 1_000_000
	MaxRooms      = 50_000
	MaxTunnels    = 250_000
)

type Room struct {
	Name string `json:"name"`
	X    int    `json:"x"`
	Y    int    `json:"y"`
}

type Tunnel struct {
	A int `json:"a"`
	B int `json:"b"`
}

type Farm struct {
	Ants      int      `json:"ants"`
	Rooms     []Room   `json:"rooms"`
	Tunnels   []Tunnel `json:"tunnels"`
	Start     int      `json:"start"`
	End       int      `json:"end"`
	Source    string   `json:"-"`
	Neighbors [][]int  `json:"-"`
}

// TunnelKey gives both directions of a tunnel the same identity.
func TunnelKey(a, b int) [2]int {
	if a > b {
		a, b = b, a
	}
	return [2]int{a, b}
}
