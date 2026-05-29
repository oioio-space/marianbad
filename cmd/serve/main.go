// Tiny static file server for dist/. Used both for local dev and for E2E tests.
package main

import (
	"flag"
	"log"
	"net/http"
	"path/filepath"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "listen address")
	dir := flag.String("dir", "dist", "directory to serve")
	flag.Parse()

	abs, err := filepath.Abs(*dir)
	if err != nil {
		log.Fatalf("abs path: %v", err)
	}
	log.Printf("serving %s on http://%s", abs, *addr)

	// Correct MIME type for .wasm — some setups still need this.
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(abs)))
	wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if filepath.Ext(r.URL.Path) == ".wasm" {
			w.Header().Set("Content-Type", "application/wasm")
		}
		w.Header().Set("Cache-Control", "no-store")
		mux.ServeHTTP(w, r)
	})

	if err := http.ListenAndServe(*addr, wrapped); err != nil {
		log.Fatal(err)
	}
}
