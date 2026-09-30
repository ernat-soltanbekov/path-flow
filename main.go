// path-flow reads one colony file and prints a minimum-turn ant schedule.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/ernat-soltanbekov/path-flow/internal/farm"
	"github.com/ernat-soltanbekov/path-flow/internal/routing"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Fprintln(out, "Usage: path-flow colony.txt\nPrint a minimum-turn schedule, topology profile, and ant load.\nVisualizer: path-flow colony.txt | visualizer -out path-flow.html")
		return 0
	}
	if len(args) != 1 {
		fmt.Fprintln(diagnostic, "ERROR: invalid data format, usage: path-flow colony.txt")
		return 1
	}
	file, err := os.Open(args[0])
	if err != nil {
		fmt.Fprintf(out, "ERROR: invalid data format, cannot open colony: %v\n", err)
		return 1
	}
	f, parseErr := farm.Parse(file)
	closeErr := file.Close()
	if parseErr != nil {
		fmt.Fprintf(out, "ERROR: invalid data format, %v\n", parseErr)
		return 1
	}
	if closeErr != nil {
		fmt.Fprintf(diagnostic, "ERROR: cannot close input: %v\n", closeErr)
		return 1
	}
	plan, err := routing.Solve(ctx, f)
	if err != nil {
		fmt.Fprintf(out, "ERROR: invalid data format, %v\n", err)
		return 1
	}
	if err := routing.Write(ctx, out, f, plan); err != nil {
		fmt.Fprintf(diagnostic, "ERROR: cannot write schedule: %v\n", err)
		return 1
	}
	return 0
}
