// Package viewer renders an offline, self-contained 3D replay.
package viewer

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"

	"github.com/ernat-soltanbekov/path-flow/internal/colony"
	"github.com/ernat-soltanbekov/path-flow/internal/replay"
)

//go:embed viewer.html
var page string

var document = template.Must(template.New("viewer").Parse(page))

// Render embeds only encoding/json output, whose default HTML escaping
// neutralizes room names containing script-closing tags or HTML markup.
func Render(w io.Writer, r *replay.Replay) error {
	if r == nil || r.Farm == nil || len(r.Turns) == 0 {
		return fmt.Errorf("cannot render an empty replay")
	}
	data, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("encode replay: %w", err)
	}
	return document.Execute(w, struct {
		Data     template.JS
		Topology string
	}{template.JS(data), colony.Classify(len(r.Farm.Rooms), len(r.Farm.Tunnels))})
}
