package farm

import (
	"errors"
	"io"
	"strings"
	"testing"
)

const valid = "3\n##start\ns 0 0\n##end\nt 2 0\na 1 1\ns-a\na-t\n"

func TestParse(t *testing.T) {
	f, err := Parse(strings.NewReader(valid))
	if err != nil {
		t.Fatal(err)
	}
	if f.Ants != 3 || len(f.Rooms) != 3 || len(f.Tunnels) != 2 || f.Start != 0 || f.End != 1 {
		t.Fatalf("unexpected graph: %+v", f)
	}
	if f.Source != valid || len(f.Neighbors[2]) != 2 {
		t.Fatal("source or adjacency not preserved")
	}
}

func TestPermittedSyntax(t *testing.T) {
	for name, input := range map[string]string{
		"CRLF and BOM":                  "\uFEFF" + strings.ReplaceAll(valid, "\n", "\r\n"),
		"no trailing newline":           strings.TrimSuffix(valid, "\n"),
		"comments and unknown commands": "# before count\n##unknown\n" + strings.Replace(valid, "##start\n", "##start\n# comment\n##startish\n\n", 1),
		"signed coordinates":            strings.Replace(valid, "s 0 0", "s -12 +35", 1),
		"shared coordinates":            strings.Replace(valid, "a 1 1", "a 0 0", 1),
		"unicode and hyphenated name":   renameRoom("таң-2008"),
		"tabs between room fields":      strings.Replace(valid, "s 0 0", "s\t0\t0", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(strings.NewReader(input)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRejectInvalidInput(t *testing.T) {
	for name, input := range map[string]string{
		"empty": "", "zero": strings.Replace(valid, "3", "0", 1),
		"negative":             strings.Replace(valid, "3", "-1", 1),
		"fraction":             strings.Replace(valid, "3", "1.5", 1),
		"too many ants":        strings.Replace(valid, "3", "1000001", 1),
		"overflow":             strings.Replace(valid, "3", "99999999999999999999999", 1),
		"no start":             strings.Replace(valid, "##start\n", "", 1),
		"no end":               strings.Replace(valid, "##end\n", "", 1),
		"unknown is not start": strings.Replace(valid, "##start", "##starting", 1),
		"command before ants":  "##start\n" + valid,
		"duplicate command":    strings.Replace(valid, "a 1 1", "##start\na 1 1", 1),
		"pending command":      strings.Replace(valid, "s 0 0\n", "", 1),
		"dangling command":     "1\n##start\ns 0 0\n##end\n",
		"command in links":     valid + "##end\n",
		"duplicate room":       strings.Replace(valid, "a 1 1", "s 1 1", 1),
		"invalid coordinate":   strings.Replace(valid, "a 1 1", "a 1.2 1", 1),
		"overflow coordinate":  strings.Replace(valid, "a 1 1", "a 99999999999999999999 1", 1),
		"L name":               renameRoom("La"),
		"name control":         renameRoom("a\x01"),
		"self loop":            valid + "s-s\n",
		"duplicate link":       valid + "s-a\n",
		"reverse duplicate":    valid + "a-s\n",
		"unknown room":         valid + "s-unknown\n",
		"extra token":          valid + "s-a junk\n",
		"room after links":     valid + "late 1 3\n",
		"NUL":                  valid + "#\x00\n", "invalid UTF8": valid + "#\xff\n",
		"long line":                  valid + "#" + strings.Repeat("x", MaxLineBytes),
		"long name":                  renameRoom(strings.Repeat("a", MaxNameBytes+1)),
		"ambiguous link":             "1\n##start\na 0 0\n##end\nb-c 2 0\na-b 1 1\nc 2 1\na-b-c\n",
		"pending followed by tunnel": "1\n##start\ns 0 0\nt 1 0\n##end\ns-t\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(strings.NewReader(input)); err == nil {
				t.Fatal("accepted malformed colony")
			}
		})
	}
}

type brokenReader struct{}

func (brokenReader) Read([]byte) (int, error) { return 0, errors.New("injected read failure") }

func TestReadFailure(t *testing.T) {
	if _, err := Parse(brokenReader{}); err == nil {
		t.Fatal("ignored read failure")
	}
}

type repeatedReader byte

func (r repeatedReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(r)
	}
	return len(p), nil
}

func TestInputLimit(t *testing.T) {
	if _, err := Parse(io.LimitReader(repeatedReader('#'), MaxInputBytes+1)); err == nil {
		t.Fatal("accepted oversized input")
	}
}

func FuzzParse(f *testing.F) {
	f.Add(valid)
	f.Add("1\n##start\na 0 0\n##end\nb 1 1\na-b")
	f.Add("##start\n\x00")
	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 1<<16 {
			t.Skip()
		}
		g, err := Parse(strings.NewReader(input))
		if err == nil && (g.Ants < 1 || g.Start < 0 || g.End < 0 || g.Start == g.End) {
			t.Fatal("accepted invalid graph")
		}
	})
}

func renameRoom(name string) string {
	return strings.NewReplacer("a 1 1", name+" 1 1", "s-a", "s-"+name, "a-t", name+"-t").Replace(valid)
}
