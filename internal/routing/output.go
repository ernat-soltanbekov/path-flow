package routing

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/ernat-soltanbekov/path-flow/internal/colony"
	"github.com/ernat-soltanbekov/path-flow/internal/farm"
)

// Write emits the input, both profile comments, and one line per turn.
// Earlier ants are printed first, so a room is vacated before its next ant
// enters even when an auditor reads a turn's moves from left to right.
func Write(ctx context.Context, out io.Writer, f *farm.Farm, p *Plan) error {
	w := bufio.NewWriterSize(out, 64<<10)
	if _, err := fmt.Fprintf(w, "%s\n# Colony profile: rooms %d | tunnels %d | topology: %s\n# Ant load: %d ants | rooms %d | load: %s\n",
		f.Source, len(f.Rooms), len(f.Tunnels), colony.Classify(len(f.Rooms), len(f.Tunnels)),
		f.Ants, len(f.Rooms), colony.ClassifyLoad(f.Ants, len(f.Rooms))); err != nil {
		return err
	}
	for turn := 1; turn <= p.Turns; turn++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		first, offset := true, 0
		for i, path := range p.Paths {
			// Ant j enters on turn j+1 and is at path[turn-j].
			for j := max(0, turn-len(path)+1); j < min(p.Counts[i], turn); j++ {
				if !first {
					if err := w.WriteByte(' '); err != nil {
						return err
					}
				}
				first = false
				if _, err := w.WriteString("L" + strconv.Itoa(offset+j+1) + "-" + f.Rooms[path[turn-j]].Name); err != nil {
					return err
				}
			}
			offset += p.Counts[i]
		}
		if err := w.WriteByte('\n'); err != nil {
			return err
		}
	}
	return w.Flush()
}
