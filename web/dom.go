//go:build js && wasm

package main

import (
	"syscall/js"
)

var (
	doc    = js.Global().Get("document")
	window = js.Global()
)

func byID(id string) js.Value {
	return doc.Call("getElementById", id)
}

func querySelectorAll(sel string) []js.Value {
	list := doc.Call("querySelectorAll", sel)
	n := list.Length()
	out := make([]js.Value, n)
	for i := 0; i < n; i++ {
		out[i] = list.Index(i)
	}
	return out
}

// on attaches a click handler and returns the js.Func so the caller can
// Release() it later. The handler is wrapped with recover() so a panic in
// game logic doesn't take down the whole WASM runtime.
func on(el js.Value, event string, fn func(this js.Value, args []js.Value)) js.Func {
	cb := js.FuncOf(func(this js.Value, args []js.Value) any {
		defer func() {
			if r := recover(); r != nil {
				js.Global().Get("console").Call("error",
					js.ValueOf("panic in handler: "), js.ValueOf(toString(r)))
			}
		}()
		fn(this, args)
		return nil
	})
	el.Call("addEventListener", event, cb)
	return cb
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if e, ok := v.(error); ok {
		return e.Error()
	}
	return "<unprintable>"
}

func setText(el js.Value, s string) {
	el.Set("textContent", s)
}

func addClass(el js.Value, c string)    { el.Get("classList").Call("add", c) }
func removeClass(el js.Value, c string) { el.Get("classList").Call("remove", c) }
func hasClass(el js.Value, c string) bool {
	return el.Get("classList").Call("contains", c).Bool()
}

func setHidden(el js.Value, hidden bool) {
	if hidden {
		addClass(el, "hidden")
	} else {
		removeClass(el, "hidden")
	}
}

func setDisabled(el js.Value, disabled bool) {
	if disabled {
		el.Set("disabled", true)
		addClass(el, "is-disabled")
	} else {
		el.Set("disabled", false)
		removeClass(el, "is-disabled")
	}
}

// setTimeoutMS schedules fn after ms milliseconds and returns the handle
// so it can be cleared.
func setTimeoutMS(ms int, fn func()) js.Value {
	cb := js.FuncOf(func(this js.Value, args []js.Value) any {
		defer func() { _ = recover() }()
		fn()
		return nil
	})
	return window.Call("setTimeout", cb, ms)
}

func clearTimeout(handle js.Value) {
	if handle.IsUndefined() || handle.IsNull() {
		return
	}
	window.Call("clearTimeout", handle)
}

// localStorageGet returns the value of the key or empty string if absent.
func localStorageGet(key string) string {
	ls := window.Get("localStorage")
	if ls.IsUndefined() || ls.IsNull() {
		return ""
	}
	v := ls.Call("getItem", key)
	if v.IsNull() || v.IsUndefined() {
		return ""
	}
	return v.String()
}

func localStorageSet(key, value string) {
	ls := window.Get("localStorage")
	if ls.IsUndefined() || ls.IsNull() {
		return
	}
	ls.Call("setItem", key, value)
}
