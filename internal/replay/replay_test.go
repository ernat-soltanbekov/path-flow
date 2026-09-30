package replay

import (
	"io"
	"strings"
	"testing"
)

const header = "2\n##start\ns 0 0\na 1 1\nb 1 2\n##end\nt 2 0\ns-a\ns-b\na-t\nb-t\na-b\ns-t\n\n# Colony profile: rooms 4 | tunnels 6 | topology: connected\n# Ant load: 2 ants | rooms 4 | load: light\n"
const movements = "L1-a L2-b\nL1-t L2-t\n"

func TestValid(t *testing.T) {
	r, err := Parse(strings.NewReader(header + movements))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Turns) != 2 || r.Turns[1][0].From != 1 || r.Turns[1][0].To != 3 {
		t.Fatalf("wrong replay: %+v", r)
	}
}

func TestRejectCorruptTrace(t *testing.T) {
	for name, trace := range map[string]string{
		"incomplete":          "L1-a\nL1-t\n",
		"ant twice":           "L1-a L1-t\n",
		"tunnel twice":        "L1-t L2-t\n",
		"occupied room":       "L1-a L2-b\nL1-b\n",
		"unknown ant":         "L3-a\n",
		"zero ant":            "L0-a\n",
		"negative ant":        "L-1-a\n",
		"noncanonical ant":    "L01-a\n",
		"unknown room":        "L1-nowhere\n",
		"nonexistent tunnel":  "L1-s\n",
		"leaves end":          "L1-t\nL1-a\n",
		"bad token":           "L1a\n",
		"interior empty turn": "L1-a\n\nL1-t\nL2-t\n",
		"opposite directions": "L1-a L2-b\nL1-b L2-a\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(strings.NewReader(header + trace)); err == nil {
				t.Fatal("accepted corrupt replay")
			}
		})
	}
	for _, input := range []string{"", header, "ERROR: invalid data format", "1\nL1-t\n"} {
		if _, err := Parse(strings.NewReader(input)); err == nil {
			t.Fatal("accepted missing header or moves")
		}
	}
}

func TestVacatedRoomAndComments(t *testing.T) {
	// Tokens are allowed in any order: ant 2 enters after ant 1 vacates a.
	input := header + "L1-a\nL2-a L1-t\n# a comment between turns\nL2-t\n\n"
	if _, err := Parse(strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
}

func FuzzReplay(f *testing.F) {
	f.Add(header + movements)
	f.Add("L1-\x00")
	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 1<<16 {
			t.Skip()
		}
		Parse(strings.NewReader(input))
	})
}

func TestMovementListIsBoundedBeforeAllocation(t *testing.T) {
	if _, err := movementTokens(strings.Repeat("L1-a ", 100000), 2); err == nil {
		t.Fatal("unbounded movement list")
	}
	if _, err := movementTokens("L1-a", 0); err == nil {
		t.Fatal("accepted movement after limit")
	}
	tokens, err := movementTokens("L1-a\t L2-b", 2)
	if err != nil || len(tokens) != 2 {
		t.Fatal("valid whitespace rejected")
	}
}

type unreadable struct{}

func (unreadable) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestReadError(t *testing.T) {
	if _, err := Parse(unreadable{}); err == nil {
		t.Fatal("ignored read failure")
	}
}
