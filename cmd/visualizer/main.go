// visualizer validates a path-flow transcript and exports an offline HTML replay.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ernat-soltanbekov/path-flow/internal/replay"
	"github.com/ernat-soltanbekov/path-flow/internal/viewer"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, input io.Reader, out, diagnostic io.Writer) int {
	flags := flag.NewFlagSet("visualizer", flag.ContinueOnError)
	flags.SetOutput(diagnostic)
	destination := flags.String("out", "path-flow.html", "self-contained HTML output file")
	address := flags.String("serve", "", "optional loopback address, e.g. 127.0.0.1:8080")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(diagnostic, "ERROR: pipe a path-flow transcript into visualizer")
		return 1
	}
	if *address != "" {
		host, _, err := net.SplitHostPort(*address)
		if err != nil || (host != "127.0.0.1" && host != "localhost" && host != "::1") {
			fmt.Fprintln(diagnostic, "ERROR: -serve requires a loopback address such as 127.0.0.1:8080")
			return 1
		}
	}
	r, err := replay.Parse(input)
	if err != nil {
		fmt.Fprintf(diagnostic, "ERROR: invalid replay, %v\n", err)
		return 1
	}
	var html bytes.Buffer
	if err := viewer.Render(&html, r); err != nil {
		fmt.Fprintf(diagnostic, "ERROR: render replay: %v\n", err)
		return 1
	}
	if err := save(*destination, html.Bytes()); err != nil {
		fmt.Fprintf(diagnostic, "ERROR: save replay: %v\n", err)
		return 1
	}
	path, err := filepath.Abs(*destination)
	if err != nil {
		path = *destination
	}
	fmt.Fprintf(out, "Replay saved: %s\n%d ants · %d turns · validated\n", path, r.Farm.Ants, len(r.Turns))
	if *address == "" {
		fmt.Fprintln(out, "Open the HTML file in a browser. No installation or internet connection is needed.")
		return 0
	}
	if err := serve(ctx, *address, html.Bytes(), out); err != nil {
		fmt.Fprintf(diagnostic, "ERROR: serve replay: %v\n", err)
		return 1
	}
	return 0
}

// Replace the destination only after a complete HTML file has been written.
func save(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".path-flow-*.html")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func serve(ctx context.Context, address string, page []byte, out io.Writer) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	defer listener.Close()
	server := &http.Server{
		Handler: handler(page), ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout: 30 * time.Second, IdleTimeout: 30 * time.Second,
	}
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			server.Shutdown(shutdown)
		case <-done:
		}
	}()
	fmt.Fprintf(out, "Open http://%s · Ctrl+C to stop\n", listener.Addr())
	err = server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func handler(page []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; base-uri 'none'; frame-ancestors 'none'")
		if r.Method == http.MethodGet {
			w.Write(page)
		}
	})
}
