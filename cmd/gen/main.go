// gen renders the templ components into dist/index.html.
package main

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"github.com/marianbad/internal/view"
)

func main() {
	out := "dist/index.html"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		log.Fatalf("mkdir: %v", err)
	}
	f, err := os.Create(out)
	if err != nil {
		log.Fatalf("create: %v", err)
	}
	defer f.Close()

	if err := view.Layout().Render(context.Background(), f); err != nil {
		log.Fatalf("render: %v", err)
	}
	log.Printf("wrote %s", out)
}
