# Marianbad — full build pipeline for Windows PowerShell.
# Runs: templ generate → render HTML → Tailwind CSS → WASM → copy runtime + fonts.

$ErrorActionPreference = "Stop"

function Step($msg) { Write-Host "==> $msg" -ForegroundColor Cyan }

# Resolve paths
$goRoot      = (go env GOROOT).Trim()
$wasmExec    = Join-Path $goRoot "lib/wasm/wasm_exec.js"
$templuiPath = (go list -m -f '{{.Dir}}' github.com/templui/templui).Trim()

# Ensure dist/ exists
$null = New-Item -ItemType Directory -Force -Path dist
$null = New-Item -ItemType Directory -Force -Path dist/assets/fonts

# 1. templ generate
Step "templ generate"
templ generate

# 2. Tailwind source globs
Step "writing Tailwind @source globs"
$sources = "@source `"../../**/*.templ`";`n@source `"$templuiPath/components/**/*.templ`";`n"
Set-Content -Path assets/css/sources.generated.css -Value $sources -Encoding utf8

# 3. Render templ → dist/index.html
Step "rendering dist/index.html"
go run ./cmd/gen dist/index.html

# 4. Tailwind v4 CLI compile
Step "compiling Tailwind CSS"
& npx "@tailwindcss/cli" -i assets/css/input.css -o dist/app.css --minify

# 5. Compile WASM (strip debug symbols + DWARF for ~30% smaller binary)
Step "building WASM"
$env:GOOS = "js"
$env:GOARCH = "wasm"
go build -ldflags "-s -w" -trimpath -o dist/main.wasm ./web
Remove-Item Env:GOOS
Remove-Item Env:GOARCH

# 6. Copy wasm_exec.js
Step "copying wasm_exec.js"
Copy-Item $wasmExec dist/wasm_exec.js -Force

# 7. Copy fonts
Step "copying fonts"
Copy-Item assets/fonts/*.woff2 dist/assets/fonts/ -Force

# 8. Standalone bundle (single file, file:// compatible)
Step "bundling dist/standalone.html"
go run ./cmd/bundle

Write-Host "`nDone." -ForegroundColor Green
Write-Host "  Multi-file version:  http://localhost:8080 (run: go run ./cmd/serve)" -ForegroundColor Gray
Write-Host "  Single-file version: dist/standalone.html (works in file://)" -ForegroundColor Gray
