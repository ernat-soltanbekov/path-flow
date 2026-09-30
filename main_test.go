package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ernat-soltanbekov/path-flow/internal/replay"
)

func TestCLIExamples(t *testing.T) {
	for name, turns := range map[string]int{"example00": 4, "example01": 5, "example02": 3, "direct": 5, "reroute": 9, "ghost-of-astana": 11} {
		t.Run(name, func(t *testing.T) {
			var out, diagnostic bytes.Buffer
			if status := run(context.Background(), []string{"examples/" + name + ".txt"}, &out, &diagnostic); status != 0 {
				t.Fatalf("status %d: %s%s", status, &out, &diagnostic)
			}
			if diagnostic.Len() != 0 {
				t.Fatal(diagnostic.String())
			}
			r, err := replay.Parse(bytes.NewReader(out.Bytes()))
			if err != nil {
				t.Fatal(err)
			}
			if len(r.Turns) != turns {
				t.Fatalf("got %d turns, want %d", len(r.Turns), turns)
			}
		})
	}
}

func TestCLIInvalidExamples(t *testing.T) {
	files, err := filepath.Glob("examples/invalid/*.txt")
	if err != nil || len(files) == 0 {
		t.Fatal("invalid fixtures missing")
	}
	files = append(files, "examples/does-not-exist.txt", t.TempDir())
	for _, path := range files {
		var out, diagnostic bytes.Buffer
		if status := run(context.Background(), []string{path}, &out, &diagnostic); status == 0 {
			t.Fatalf("accepted %s", path)
		}
		if !strings.HasPrefix(out.String(), "ERROR: invalid data format") {
			t.Fatalf("wrong error: %s%s", &out, &diagnostic)
		}
		if strings.Contains(out.String(), "# Colony profile:") {
			t.Fatal("partial success output on invalid input")
		}
	}
}

func TestCLIUsage(t *testing.T) {
	for _, args := range [][]string{nil, {"a", "b"}} {
		if status := run(context.Background(), args, io.Discard, io.Discard); status == 0 {
			t.Fatal("accepted wrong arguments")
		}
	}
	for _, arg := range []string{"-h", "--help"} {
		var out bytes.Buffer
		if status := run(context.Background(), []string{arg}, &out, io.Discard); status != 0 || !strings.Contains(out.String(), "Usage:") {
			t.Fatal("help missing")
		}
	}
}

type brokenOutput struct{}

func (brokenOutput) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestCLIWriteErrorAndCancellation(t *testing.T) {
	var diagnostic bytes.Buffer
	if run(context.Background(), []string{"examples/direct.txt"}, brokenOutput{}, &diagnostic) == 0 {
		t.Fatal("ignored write failure")
	}
	if !strings.Contains(diagnostic.String(), "cannot write") {
		t.Fatal(diagnostic.String())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if run(ctx, []string{"examples/direct.txt"}, io.Discard, io.Discard) == 0 {
		t.Fatal("ignored cancellation")
	}
}

func TestTopologyOutput(t *testing.T) {
	for name, label := range map[string]string{"direct": "sparse", "boundary-1.2": "connected", "boundary-2.0": "connected", "dense": "dense"} {
		var out bytes.Buffer
		if run(context.Background(), []string{"examples/" + name + ".txt"}, &out, io.Discard) != 0 {
			t.Fatal(out.String())
		}
		if !strings.Contains(out.String(), "topology: "+label+"\n") {
			t.Fatal(out.String())
		}
		if strings.Index(out.String(), "# Ant load:") > strings.Index(out.String(), "\nL1-") {
			t.Fatal("profile after movements")
		}
	}
}

func TestNoTemporaryFilesNeeded(t *testing.T) {
	// The CLI only needs its input and a writable stdout, not a working folder.
	path := filepath.Join(t.TempDir(), "colony.txt")
	if err := os.WriteFile(path, []byte("1\n##start\ns 0 0\n##end\nt 1 1\ns-t\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if run(context.Background(), []string{path}, io.Discard, io.Discard) != 0 {
		t.Fatal("standalone input failed")
	}
}
