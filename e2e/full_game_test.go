package e2e

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// TestPlayFullTwoPlayerGame plays a deterministic 2-player game to completion
// and verifies the win banner appears + the score is incremented.
func TestPlayFullTwoPlayerGame(t *testing.T) {
	base, stop := startServer(t)
	defer stop()

	ctx, logs, cleanup := chromeCtx(t, 30*time.Second)
	defer cleanup()

	// Start a fresh, predictable state by clearing localStorage.
	resetState := chromedp.Tasks{
		chromedp.Navigate(base + "/"),
		chromedp.Evaluate(`localStorage.clear()`, nil),
		chromedp.Reload(),
		chromedp.WaitVisible(`#mode-2p`, chromedp.ByQuery),
		chromedp.Sleep(400 * time.Millisecond),
	}

	if err := chromedp.Run(ctx, resetState); err != nil {
		t.Fatalf("reset: %v", err)
	}

	// 2P mode → predictable, no AI.
	if err := chromedp.Run(ctx,
		chromedp.Click(`#mode-2p`, chromedp.ByQuery),
		chromedp.WaitVisible(`#board`, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("open 2p: %v", err)
	}

	// Play a sequence of moves that empties the board.
	// Strategy: empty the rows one at a time, the last mover loses (misère).
	// Move list: each (row, idx) clicks card at idx, removes cards idx..end.
	moves := []struct {
		row int
		idx int
	}{
		{2, 0}, // remove row 2 (3 cards)
		{1, 0}, // remove row 1 (5 cards)
		{0, 6}, // remove last card of row 0 (1)
		{0, 5}, // remove next (1)
		{0, 4},
		{0, 3},
		{0, 2},
		{0, 1},
		{0, 0}, // takes the last → this player loses
	}

	for i, m := range moves {
		sel := jsCardSel(m.row, m.idx)
		validate := `#btn-validate:not([disabled])`
		if err := chromedp.Run(ctx,
			chromedp.Click(sel, chromedp.ByQuery),
			chromedp.WaitVisible(validate, chromedp.ByQuery),
			chromedp.Click(`#btn-validate`, chromedp.ByQuery),
			chromedp.Sleep(150*time.Millisecond),
		); err != nil {
			t.Fatalf("move %d (row %d idx %d): %v\nconsole:\n%s",
				i, m.row, m.idx, err, strings.Join(*logs, "\n"))
		}
	}

	// After the final move, status should announce a winner.
	var status string
	if err := chromedp.Run(ctx,
		chromedp.Sleep(300*time.Millisecond),
		chromedp.Text(`#status`, &status, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("read status: %v", err)
	}
	if !strings.Contains(status, "gagné") {
		t.Fatalf("expected win message in status, got %q\nconsole:\n%s",
			status, strings.Join(*logs, "\n"))
	}

	// Victory modal should be visible (fireworks triggered).
	var modalVisible bool
	if err := chromedp.Run(ctx,
		chromedp.Sleep(200*time.Millisecond),
		chromedp.Evaluate(
			`!document.getElementById('victory-modal').classList.contains('hidden')`,
			&modalVisible),
	); err != nil {
		t.Fatalf("modal check: %v", err)
	}
	if !modalVisible {
		t.Error("expected victory modal to be visible after a player wins")
	}

	// Verify the fireworks canvas was actually created with non-zero size
	// (regression test for the bug where Fireworks ran before layout settled).
	var canvasWidth int
	if err := chromedp.Run(ctx,
		chromedp.Sleep(400*time.Millisecond), // let RAF + fireworks instantiate
		chromedp.Evaluate(
			`(function(){var c=document.querySelector('#fireworks-overlay canvas');return c?c.width:0})()`,
			&canvasWidth),
	); err != nil {
		t.Fatalf("canvas check: %v", err)
	}
	if canvasWidth < 100 {
		t.Errorf("fireworks canvas width = %d, expected ≥ 100 (canvas missing or zero-sized)", canvasWidth)
	}

	// Score should be 0-1 or 1-0.
	var score string
	if err := chromedp.Run(ctx,
		chromedp.Text(`#score-display`, &score, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("read score: %v", err)
	}
	if !strings.Contains(score, "1") {
		t.Errorf("expected a '1' in score after one game, got %q", score)
	}
}

// TestScreenshotInitial captures a screenshot of the mode selector,
// useful for visual review. The PNG is written next to the test binary.
func TestScreenshotInitial(t *testing.T) {
	if os.Getenv("E2E_SCREENSHOTS") == "" {
		t.Skip("set E2E_SCREENSHOTS=1 to enable")
	}
	base, stop := startServer(t)
	defer stop()

	ctx, _, cleanup := chromeCtx(t, 15*time.Second)
	defer cleanup()

	var buf []byte
	err := chromedp.Run(ctx,
		chromedp.EmulateViewport(1024, 768),
		chromedp.Navigate(base+"/"),
		chromedp.WaitVisible(`#mode-vsai`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.FullScreenshot(&buf, 90),
	)
	if err != nil {
		t.Fatalf("screenshot: %v", err)
	}
	out := filepath.Join("..", "docs", "screenshots", "mode-selector.png")
	_ = os.MkdirAll(filepath.Dir(out), 0o755)
	if err := os.WriteFile(out, buf, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("screenshot: %s (%d bytes)", out, len(buf))
}

// Verify no JS console errors during a quick full play-through.
func TestNoConsoleErrorsDuringPlay(t *testing.T) {
	base, stop := startServer(t)
	defer stop()

	ctx, logs, cleanup := chromeCtx(t, 20*time.Second)
	defer cleanup()

	err := chromedp.Run(ctx,
		chromedp.Navigate(base+"/"),
		chromedp.Evaluate(`localStorage.clear()`, nil),
		chromedp.Reload(),
		chromedp.WaitVisible(`#mode-vsai`, chromedp.ByQuery),
		chromedp.Sleep(400*time.Millisecond),
		chromedp.Click(`#mode-vsai`, chromedp.ByQuery),
		chromedp.WaitVisible(`#board`, chromedp.ByQuery),
		// 1 or 2 moves with AI replies; even if game doesn't end, we just
		// want to catch any JS error.
		chromedp.Sleep(2*time.Second),
	)
	if err != nil {
		t.Fatalf("flow: %v", err)
	}
	if hasJSError(*logs) {
		t.Fatalf("console errors:\n%s", strings.Join(*logs, "\n"))
	}
	for _, l := range *logs {
		t.Logf("console: %s", l)
	}
}

func jsCardSel(row, idx int) string {
	return `.card[data-row="` + itoa(row) + `"][data-idx="` + itoa(idx) + `"]:not(.removed)`
}

func itoa(n int) string {
	switch n {
	case 0:
		return "0"
	case 1:
		return "1"
	case 2:
		return "2"
	case 3:
		return "3"
	case 4:
		return "4"
	case 5:
		return "5"
	case 6:
		return "6"
	}
	return ""
}

// Surface context as variable to silence unused warning if test list changes.
var _ context.Context = context.Background()
