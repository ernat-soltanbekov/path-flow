package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const transcript = "1\n##start\ns 0 0\n##end\nt 1 1\ns-t\n\nL1-t\n"

func TestExport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "replay.html")
	var out, diagnostic bytes.Buffer
	if run(context.Background(), []string{"-out", path}, strings.NewReader(transcript), &out, &diagnostic) != 0 {
		t.Fatal(diagnostic.String())
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Contains(data, []byte("PATH FLOW")) {
		t.Fatalf("output missing: %v", err)
	}
	if !strings.Contains(out.String(), "1 ants · 1 turns · validated") {
		t.Fatal(out.String())
	}
}

func TestExportFailurePreservesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "replay.html")
	if err := os.WriteFile(path, []byte("existing file"), 0600); err != nil {
		t.Fatal(err)
	}
	if run(context.Background(), []string{"-out", path}, strings.NewReader("invalid"), io.Discard, io.Discard) == 0 {
		t.Fatal("accepted bad replay")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "existing file" {
		t.Fatal("overwrote file on invalid input")
	}
	if err := save(filepath.Join(path, "impossible"), nil); err == nil {
		t.Fatal("ignored filesystem error")
	}
}

func TestFlags(t *testing.T) {
	for _, args := range [][]string{{"-unknown"}, {"extra"}, {"-serve", "0.0.0.0:8080"}, {"-serve", "bad"}} {
		if run(context.Background(), args, strings.NewReader(transcript), io.Discard, io.Discard) == 0 {
			t.Fatalf("accepted %v", args)
		}
	}
	if run(context.Background(), []string{"-h"}, strings.NewReader(""), io.Discard, io.Discard) != 0 {
		t.Fatal("help failed")
	}
}

func TestHTTPHandler(t *testing.T) {
	h := handler([]byte("test page"))
	for _, tc := range []struct {
		method, path string
		status       int
	}{{"GET", "/", 200}, {"HEAD", "/", 200}, {"POST", "/", 405}, {"GET", "/missing", 404}} {
		r := httptest.NewRecorder()
		h.ServeHTTP(r, httptest.NewRequest(tc.method, tc.path, nil))
		if r.Code != tc.status {
			t.Fatalf("%s %s: %d", tc.method, tc.path, r.Code)
		}
		if tc.method == http.MethodHead && r.Body.Len() != 0 {
			t.Fatal("HEAD includes body")
		}
		if tc.status == 200 && r.Header().Get("Content-Security-Policy") == "" {
			t.Fatal("missing CSP")
		}
	}
}
