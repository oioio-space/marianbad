//go:build js && wasm

package main

func main() {
	c := New()
	c.Mount()
	// Keep the Go runtime alive so registered DOM callbacks keep firing.
	select {}
}
