package e2e

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestScreenshotVictory(t *testing.T) {
	if os.Getenv("E2E_SCREENSHOTS") == "" {
		t.Skip("set E2E_SCREENSHOTS=1 to enable")
	}
	base, stop := startServer(t)
	defer stop()

	ctx, _, cleanup := chromeCtx(t, 60*time.Second)
	defer cleanup()

	moves := []struct{ row, idx int }{
		{2, 0}, {1, 0},
		{0, 6}, {0, 5}, {0, 4}, {0, 3}, {0, 2}, {0, 1}, {0, 0},
	}

	tasks := chromedp.Tasks{
		chromedp.EmulateViewport(1024, 768),
		chromedp.Navigate(base + "/"),
		chromedp.Evaluate(`localStorage.clear()`, nil),
		chromedp.Reload(),
		chromedp.WaitVisible(`#mode-2p`, chromedp.ByQuery),
		chromedp.Sleep(400 * time.Millisecond),
		chromedp.Click(`#mode-2p`, chromedp.ByQuery),
		chromedp.WaitVisible(`#board`, chromedp.ByQuery),
	}
	for _, m := range moves {
		tasks = append(tasks,
			chromedp.Click(jsCardSel(m.row, m.idx), chromedp.ByQuery),
			chromedp.WaitEnabled(`#btn-validate`, chromedp.ByQuery),
			chromedp.Click(`#btn-validate`, chromedp.ByQuery),
			chromedp.Sleep(100*time.Millisecond),
		)
	}

	var buf []byte
	tasks = append(tasks,
		chromedp.Sleep(1500*time.Millisecond),
		chromedp.FullScreenshot(&buf, 90),
	)

	if err := chromedp.Run(ctx, tasks); err != nil {
		t.Fatalf("flow: %v", err)
	}
	out := filepath.Join("..", "docs", "screenshots", "victory.png")
	_ = os.MkdirAll(filepath.Dir(out), 0o755)
	if err := os.WriteFile(out, buf, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("victory screenshot: %s (%d bytes)", out, len(buf))
}
