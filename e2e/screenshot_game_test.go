package e2e

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestScreenshotGame(t *testing.T) {
	if os.Getenv("E2E_SCREENSHOTS") == "" {
		t.Skip("set E2E_SCREENSHOTS=1 to enable")
	}
	base, stop := startServer(t)
	defer stop()

	ctx, _, cleanup := chromeCtx(t, 20*time.Second)
	defer cleanup()

	var buf []byte
	err := chromedp.Run(ctx,
		chromedp.EmulateViewport(1024, 768),
		chromedp.Navigate(base+"/"),
		chromedp.Evaluate(`localStorage.clear()`, nil),
		chromedp.Reload(),
		chromedp.WaitVisible(`#mode-2p`, chromedp.ByQuery),
		chromedp.Sleep(400*time.Millisecond),
		chromedp.Click(`#mode-2p`, chromedp.ByQuery),
		chromedp.WaitVisible(`#board`, chromedp.ByQuery),
		// Select a card to show the "selected" state.
		chromedp.Sleep(300*time.Millisecond),
		chromedp.Click(`.card[data-row="1"][data-idx="2"]`, chromedp.ByQuery),
		chromedp.Sleep(300*time.Millisecond),
		chromedp.FullScreenshot(&buf, 90),
	)
	if err != nil {
		t.Fatalf("screenshot: %v", err)
	}
	out := filepath.Join("..", "docs", "screenshots", "game-view.png")
	_ = os.MkdirAll(filepath.Dir(out), 0o755)
	if err := os.WriteFile(out, buf, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("screenshot: %s (%d bytes)", out, len(buf))
}
