# Marianbad — build pipeline (Linux/macOS).
# Windows users: run `pwsh ./build.ps1`.

GOROOT ?= $(shell go env GOROOT)
WASM_EXEC := $(GOROOT)/lib/wasm/wasm_exec.js
TEMPLUI_PATH := $(shell go list -m -f '{{.Dir}}' github.com/templui/templui)

.PHONY: all gen sources tailwind wasm runtime fonts html test e2e serve clean

all: html tailwind wasm runtime fonts bundle

bundle: html tailwind wasm runtime fonts
	go run ./cmd/bundle

gen:
	templ generate

sources:
	@printf '@source "../../**/*.templ";\n@source "$(TEMPLUI_PATH)/components/**/*.templ";\n' > assets/css/sources.generated.css

html: gen
	go run ./cmd/gen dist/index.html

tailwind: sources
	npx @tailwindcss/cli -i assets/css/input.css -o dist/app.css --minify

wasm:
	GOOS=js GOARCH=wasm go build -ldflags "-s -w" -trimpath -o dist/main.wasm ./web

runtime:
	mkdir -p dist
	cp "$(WASM_EXEC)" dist/wasm_exec.js

fonts:
	mkdir -p dist/assets/fonts
	cp assets/fonts/*.woff2 dist/assets/fonts/

test:
	go test ./internal/...

e2e: all
	go test ./e2e/...

serve: all
	@echo "Serving dist/ on http://localhost:8080 (Ctrl+C to stop)"
	go run ./cmd/serve

clean:
	rm -rf dist/*.css dist/*.html dist/*.wasm dist/*.js dist/assets
