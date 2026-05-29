// Package e2e drives the built dist/ bundle with headless Chrome via chromedp.
//
// These tests REQUIRE:
//   - a built dist/ (run `make` or `pwsh build.ps1` first)
//   - Chrome installed (path read from env CHROME_PATH or default)
//
// Run with:  go test ./e2e/... -v
package e2e

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

const defaultChromePath = `C:\Program Files\Google\Chrome\Application\chrome.exe`

func chromePath() string {
	if p := os.Getenv("CHROME_PATH"); p != "" {
		return p
	}
	return defaultChromePath
}

// startServer spins up an http server serving the project's dist/ directory
// on a free localhost port. Returns the base URL and a shutdown func.
func startServer(t *testing.T) (string, func()) {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	// tests run from project root or e2e/ — handle both.
	dist := filepath.Join(wd, "dist")
	if _, err := os.Stat(dist); err != nil {
		dist = filepath.Join(wd, "..", "dist")
		if _, err := os.Stat(dist); err != nil {
			t.Fatalf("cannot find dist/ from %s", wd)
		}
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	mux := http.NewServeMux()
	fs := http.FileServer(http.Dir(dist))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if filepath.Ext(r.URL.Path) == ".wasm" {
			w.Header().Set("Content-Type", "application/wasm")
		}
		w.Header().Set("Cache-Control", "no-store")
		fs.ServeHTTP(w, r)
	})
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)

	return "http://" + ln.Addr().String(), func() { srv.Close() }
}

// chromeCtx returns a chromedp context with a console-capture installed.
// The returned `logs` slice collects stdout/stderr from page console.
func chromeCtx(t *testing.T, timeout time.Duration) (context.Context, *[]string, func()) {
	t.Helper()

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(chromePath()),
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-gpu", true),
	)
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)

	ctx, cxCancel := chromedp.NewContext(allocCtx)
	tCtx, tCancel := context.WithTimeout(ctx, timeout)

	var (
		mu   sync.Mutex
		logs []string
	)
	chromedp.ListenTarget(ctx, func(ev any) {
		switch e := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			var b strings.Builder
			b.WriteString("[" + string(e.Type) + "] ")
			for i, a := range e.Args {
				if i > 0 {
					b.WriteString(" ")
				}
				if a.Value != nil {
					b.Write(a.Value)
				} else {
					b.WriteString(a.Description)
				}
			}
			mu.Lock()
			logs = append(logs, b.String())
			mu.Unlock()
		case *runtime.EventExceptionThrown:
			mu.Lock()
			logs = append(logs, "[exception] "+e.ExceptionDetails.Error())
			mu.Unlock()
		}
	})

	cleanup := func() {
		tCancel()
		cxCancel()
		allocCancel()
	}
	return tCtx, &logs, cleanup
}

func TestPageLoadsAndShowsModeSelector(t *testing.T) {
	base, stop := startServer(t)
	defer stop()

	ctx, logs, cleanup := chromeCtx(t, 15*time.Second)
	defer cleanup()

	var modeBtnVisible bool
	err := chromedp.Run(ctx,
		chromedp.Navigate(base+"/"),
		chromedp.WaitVisible(`#mode-selector`, chromedp.ByQuery),
		chromedp.Evaluate(`!!document.getElementById('mode-vsai')`, &modeBtnVisible),
	)
	if err != nil {
		t.Fatalf("navigate/wait: %v\nconsole:\n%s", err, strings.Join(*logs, "\n"))
	}
	if !modeBtnVisible {
		t.Fatalf("mode-vsai button not in DOM\nconsole:\n%s", strings.Join(*logs, "\n"))
	}
	// Surface any errors at least once for visibility.
	for _, l := range *logs {
		t.Logf("console: %s", l)
	}
}

func TestSelectVsAIShowsBoard(t *testing.T) {
	base, stop := startServer(t)
	defer stop()

	ctx, logs, cleanup := chromeCtx(t, 20*time.Second)
	defer cleanup()

	var cardCount int
	err := chromedp.Run(ctx,
		chromedp.Navigate(base+"/"),
		chromedp.WaitVisible(`#mode-vsai`, chromedp.ByQuery),
		// Give the WASM a beat to attach handlers before clicking.
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Click(`#mode-vsai`, chromedp.ByQuery),
		chromedp.WaitVisible(`#board`, chromedp.ByQuery),
		chromedp.Evaluate(`document.querySelectorAll('#board .card').length`, &cardCount),
	)
	if err != nil {
		t.Fatalf("vsai flow: %v\nconsole:\n%s", err, strings.Join(*logs, "\n"))
	}
	if cardCount != 15 {
		t.Errorf("expected 15 cards, got %d\nconsole:\n%s", cardCount, strings.Join(*logs, "\n"))
	}
}

func TestPlayOneHumanMove(t *testing.T) {
	base, stop := startServer(t)
	defer stop()

	ctx, logs, cleanup := chromeCtx(t, 25*time.Second)
	defer cleanup()

	// We pick 2-player mode so the move is processed without an AI step
	// (deterministic, no timing).
	var removedCount int
	err := chromedp.Run(ctx,
		chromedp.Navigate(base+"/"),
		chromedp.WaitVisible(`#mode-2p`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Click(`#mode-2p`, chromedp.ByQuery),
		chromedp.WaitVisible(`#board`, chromedp.ByQuery),
		// Click the first card in the top row (row 0, idx 0) — sélection 7 cards.
		chromedp.Click(`.card[data-row="0"][data-idx="0"]`, chromedp.ByQuery),
		// Validate.
		chromedp.WaitEnabled(`#btn-validate`, chromedp.ByQuery),
		chromedp.Click(`#btn-validate`, chromedp.ByQuery),
		chromedp.Sleep(400*time.Millisecond),
		chromedp.Evaluate(`document.querySelectorAll('#board .card.removed').length`, &removedCount),
	)
	if err != nil {
		t.Fatalf("play move: %v\nconsole:\n%s", err, strings.Join(*logs, "\n"))
	}
	// After selecting idx 0 of row 0 (which has 7 cards), all 7 should be removed.
	if removedCount != 7 {
		t.Errorf("expected 7 removed cards, got %d", removedCount)
	}
	if !strings.Contains(consoleConcat(*logs), "") {
		// no-op; just keep logs reachable.
	}
	if hasJSError(*logs) {
		t.Fatalf("page logged JS error:\n%s", strings.Join(*logs, "\n"))
	}
}

func consoleConcat(logs []string) string { return strings.Join(logs, "\n") }

func hasJSError(logs []string) bool {
	for _, l := range logs {
		if strings.HasPrefix(l, "[exception]") || strings.HasPrefix(l, "[error]") {
			return true
		}
	}
	return false
}

// guard against missing chrome in CI etc.
var _ = errors.New
var _ = fmt.Sprintf
