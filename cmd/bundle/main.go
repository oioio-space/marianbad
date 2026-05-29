// bundle reads the multi-file dist/ output and produces a single
// self-contained HTML file (dist/standalone.html) usable directly from file://.
//
// It inlines:
//   - wasm_exec.js as <script>
//   - app.css as <style>, with font files embedded as base64 data URLs
//   - main.wasm as base64 in a <script>, decoded at load time into a Uint8Array
//     and passed to WebAssembly.instantiate (no fetch() call).
package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func main() {
	dist := flag.String("dist", "dist", "dist directory to read inputs from")
	out := flag.String("out", "dist/standalone.html", "output file")
	flag.Parse()

	htmlBytes := must(os.ReadFile(filepath.Join(*dist, "index.html")))
	cssBytes := must(os.ReadFile(filepath.Join(*dist, "app.css")))
	jsBytes := must(os.ReadFile(filepath.Join(*dist, "wasm_exec.js")))
	wasmBytes := must(os.ReadFile(filepath.Join(*dist, "main.wasm")))
	fwBytes := must(os.ReadFile(filepath.Join(*dist, "fireworks.js")))

	html := string(htmlBytes)
	css := string(cssBytes)
	js := string(jsBytes)
	fw := string(fwBytes)

	// 1. Embed font files into the CSS as data URLs.
	css = inlineFonts(css, filepath.Join(*dist, "assets", "fonts"))

	// 2. Replace <link rel="stylesheet" href="app.css"> with <style>...</style>.
	html = replaceCSSLink(html, css)

	// 3. Inline fireworks.js and wasm_exec.js.
	html = replaceScriptSrc(html, "fireworks.js", fw)
	html = replaceScriptSrc(html, "wasm_exec.js", js)

	// 4. Replace the WASM loader script with an inline base64 loader.
	html = replaceWasmLoader(html, wasmBytes)

	if err := os.WriteFile(*out, []byte(html), 0o644); err != nil {
		log.Fatalf("write %s: %v", *out, err)
	}
	stat := must(os.Stat(*out))
	log.Printf("wrote %s (%.2f MB)", *out, float64(stat.Size())/(1024*1024))
}

func must[T any](v T, err error) T {
	if err != nil {
		log.Fatal(err)
	}
	return v
}

// inlineFonts rewrites url("../fonts/foo.woff2") references in the CSS to
// data: URIs by reading each font file from fontsDir.
func inlineFonts(css, fontsDir string) string {
	// Matches url("...woff2") or url(...woff2) or url('...woff2').
	re := regexp.MustCompile(`url\(["']?([^"')]+\.woff2)["']?\)`)
	return re.ReplaceAllStringFunc(css, func(match string) string {
		sub := re.FindStringSubmatch(match)
		if len(sub) != 2 {
			return match
		}
		ref := sub[1]
		// Strip any path prefix; we resolve by basename in fontsDir.
		name := filepath.Base(ref)
		path := filepath.Join(fontsDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("WARN: cannot inline font %s (%v) — leaving as-is", path, err)
			return match
		}
		enc := base64.StdEncoding.EncodeToString(data)
		return fmt.Sprintf(`url("data:font/woff2;base64,%s")`, enc)
	})
}

func replaceCSSLink(html, css string) string {
	re := regexp.MustCompile(`<link[^>]+href=["']app\.css["'][^>]*>`)
	tag := "<style>\n" + css + "\n</style>"
	return re.ReplaceAllString(html, tag)
}

func replaceScriptSrc(html, src, body string) string {
	pattern := `<script[^>]+src=["']` + regexp.QuoteMeta(src) + `["'][^>]*></script>`
	re := regexp.MustCompile(pattern)
	return re.ReplaceAllString(html, "<script>\n"+body+"\n</script>")
}

// replaceWasmLoader rewrites the inline loader script with one that
// decodes embedded gzipped base64 instead of calling fetch(). The compressed
// payload is decoded via the browser's native DecompressionStream API
// (supported in Chrome 80+, Firefox 113+, Safari 16.4+).
func replaceWasmLoader(html string, wasm []byte) string {
	// Compress with gzip at max level.
	var buf bytes.Buffer
	gz, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		log.Fatalf("gzip writer: %v", err)
	}
	if _, err := gz.Write(wasm); err != nil {
		log.Fatalf("gzip write: %v", err)
	}
	if err := gz.Close(); err != nil {
		log.Fatalf("gzip close: %v", err)
	}
	enc := base64.StdEncoding.EncodeToString(buf.Bytes())
	log.Printf("wasm %d B → gzip %d B → base64 %d B (%.1f%% of raw)",
		len(wasm), buf.Len(), len(enc), 100*float64(len(enc))/float64(len(wasm)))

	loader := `<script>
(function () {
    if (typeof WebAssembly === "undefined" || typeof Go === "undefined") {
        var el = document.getElementById("wasm-error");
        if (el) {
            el.textContent = "WebAssembly indisponible dans ce navigateur.";
            el.classList.remove("hidden");
        }
        return;
    }
    if (typeof DecompressionStream === "undefined") {
        var el = document.getElementById("wasm-error");
        if (el) {
            el.textContent = "Ce navigateur est trop ancien (DecompressionStream requis).";
            el.classList.remove("hidden");
        }
        return;
    }
    var b64 = "__WASM_GZ_B64__";
    var bin = atob(b64);
    var compressed = new Uint8Array(bin.length);
    for (var i = 0; i < bin.length; i++) compressed[i] = bin.charCodeAt(i);

    var stream = new Blob([compressed]).stream().pipeThrough(new DecompressionStream("gzip"));
    new Response(stream).arrayBuffer()
        .then(function (wasmBytes) {
            var go = new Go();
            return WebAssembly.instantiate(wasmBytes, go.importObject)
                .then(function (result) { go.run(result.instance); });
        })
        .catch(function (err) {
            console.error("WASM init failed:", err);
            var el = document.getElementById("wasm-error");
            if (el) {
                el.textContent = "Erreur d'initialisation : " + err.message;
                el.classList.remove("hidden");
            }
        });
})();
</script>`
	loader = strings.Replace(loader, "__WASM_GZ_B64__", enc, 1)

	re := regexp.MustCompile(`(?s)<script>\s*\(function \(\) \{[^<]*WebAssembly\.instantiateStreaming.*?\}\)\(\);\s*</script>`)
	if !re.MatchString(html) {
		log.Fatal("could not locate the WASM loader script to replace")
	}
	return re.ReplaceAllLiteralString(html, loader)
}
