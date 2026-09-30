package viewer

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/ernat-soltanbekov/path-flow/internal/replay"
)

func TestRenderEscapesRoomNames(t *testing.T) {
	name := "</script><script>alert(1)</script>"
	input := "1\n##start\ns 0 0\n##end\n" + name + " 1 1\ns-" + name + "\n\nL1-" + name + "\n"
	r, err := replay.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := Render(&output, r); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), name) {
		t.Fatal("HTML injection not escaped")
	}
	if !strings.Contains(output.String(), `\u003c/script\u003e`) {
		t.Fatal("safe JSON missing")
	}
	if !strings.Contains(output.String(), `id="replay-data"`) {
		t.Fatal("replay data missing")
	}
}

func TestRejectEmpty(t *testing.T) {
	if err := Render(io.Discard, nil); err == nil {
		t.Fatal("accepted empty replay")
	}
}
