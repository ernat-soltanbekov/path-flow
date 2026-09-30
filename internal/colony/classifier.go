// Package colony describes the topology and load of a parsed ant colony.
// The classifier is deliberately rule based; it uses no model or external service.
package colony

// ConnectivityRatio returns tunnels / rooms. Invalid counts return zero.
func ConnectivityRatio(rooms, tunnels int) float64 {
	if rooms <= 0 || tunnels < 0 {
		return 0
	}
	return float64(tunnels) / float64(rooms)
}

// Classify labels the tunnel-to-room ratio. Both boundaries are inclusive
// for connected: exactly 1.2 and exactly 2.0 belong to that category.
func Classify(rooms, tunnels int) string {
	ratio := ConnectivityRatio(rooms, tunnels)
	switch {
	case ratio < 1.2:
		return "sparse"
	case ratio <= 2.0:
		return "connected"
	default:
		return "dense"
	}
}

// LoadRatio returns ants / rooms. Invalid counts return zero.
func LoadRatio(ants, rooms int) float64 {
	if rooms <= 0 || ants < 0 {
		return 0
	}
	return float64(ants) / float64(rooms)
}

// ClassifyLoad labels the number of ants relative to the number of rooms.
func ClassifyLoad(ants, rooms int) string {
	ratio := LoadRatio(ants, rooms)
	switch {
	case ratio < 1:
		return "light"
	case ratio <= 3:
		return "moderate"
	default:
		return "heavy"
	}
}
