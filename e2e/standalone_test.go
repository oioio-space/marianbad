package e2e

import (
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// TestStandaloneFileURL verifies dist/standalone.html plays via file:// — the
// scenario that motivated bundling everything inline.
func TestStandaloneFileURL(t *testing.T) {
	abs, err := filepath.Abs(filepath.Join("..", "dist", "standalone.html"))
	if err != nil {
		t.Fatal(err)
	}
	fileURL := "file:///" + filepath.ToSlash(abs)
	// Browsers expect properly encoded file URLs.
	if u, err := url.Parse(fileURL); err == nil {
		fileURL = u.String()
	}

	ctx, logs, cleanup := chromeCtx(t, 30*time.Second)
	defer cleanup()

	var cardCount int
	err = chromedp.Run(ctx,
		chromedp.Navigate(fileURL),
		chromedp.WaitVisible(`#mode-2p`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Click(`#mode-2p`, chromedp.ByQuery),
		chromedp.WaitVisible(`#board`, chromedp.ByQuery),
		chromedp.Evaluate(`document.querySelectorAll('#board .card').length`, &cardCount),
	)
	if err != nil {
		t.Fatalf("standalone via file://: %v\nconsole:\n%s", err, strings.Join(*logs, "\n"))
	}
	if cardCount != 15 {
		t.Errorf("expected 15 cards, got %d", cardCount)
	}
	if hasJSError(*logs) {
		t.Fatalf("JS errors:\n%s", strings.Join(*logs, "\n"))
	}
}
