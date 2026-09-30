package colony

import "testing"

func TestTopologyBoundaries(t *testing.T) {
	for _, tc := range []struct {
		rooms, tunnels int
		ratio          float64
		label          string
	}{
		{5, 0, 0, "sparse"}, {5, 5, 1, "sparse"},
		{5, 6, 1.2, "connected"}, {5, 10, 2, "connected"},
		{5, 11, 2.2, "dense"}, {6, 13, 13.0 / 6, "dense"},
		{0, 2, 0, "sparse"}, {-1, 2, 0, "sparse"}, {2, -1, 0, "sparse"},
	} {
		if got := ConnectivityRatio(tc.rooms, tc.tunnels); got != tc.ratio {
			t.Errorf("ratio(%d,%d) = %v, want %v", tc.rooms, tc.tunnels, got, tc.ratio)
		}
		if got := Classify(tc.rooms, tc.tunnels); got != tc.label {
			t.Errorf("classify(%d,%d) = %q, want %q", tc.rooms, tc.tunnels, got, tc.label)
		}
	}
}

func TestLoadBoundaries(t *testing.T) {
	for _, tc := range []struct {
		ants, rooms int
		ratio       float64
		label       string
	}{
		{0, 6, 0, "light"}, {5, 6, 5.0 / 6, "light"},
		{6, 6, 1, "moderate"}, {9, 6, 1.5, "moderate"},
		{18, 6, 3, "moderate"}, {19, 6, 19.0 / 6, "heavy"},
		{9, 0, 0, "light"}, {-1, 6, 0, "light"}, {1, -1, 0, "light"},
	} {
		if got := LoadRatio(tc.ants, tc.rooms); got != tc.ratio {
			t.Errorf("load ratio(%d,%d) = %v, want %v", tc.ants, tc.rooms, got, tc.ratio)
		}
		if got := ClassifyLoad(tc.ants, tc.rooms); got != tc.label {
			t.Errorf("load(%d,%d) = %q, want %q", tc.ants, tc.rooms, got, tc.label)
		}
	}
}
