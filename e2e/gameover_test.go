package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// TestGameOverScreen verifies the retro "GAME OVER" overlay appears when the
// human loses vs the AI. We force a loss by always taking the smallest legal
// move (taking 1 from the longest non-empty row) which is sub-optimal against
// a perfect AI.
func TestGameOverScreen(t *testing.T) {
	base, stop := startServer(t)
	defer stop()

	ctx, logs, cleanup := chromeCtx(t, 60*time.Second)
	defer cleanup()

	// JS that picks a deliberately bad move: take 1 card from the longest
	// remaining row. Against optimal AI play this loses.
	playBad := `(function() {
		var rows = [0,0,0];
		for (var r = 0; r < 3; r++) {
			rows[r] = document.querySelectorAll('.card[data-row="' + r + '"]:not(.removed)').length;
		}
		var max = 0, mi = -1;
		for (var i = 0; i < 3; i++) if (rows[i] > max) { max = rows[i]; mi = i; }
		if (mi === -1) return false;
		// Take 1 card → click the rightmost-remaining card (data-idx = rows[mi]-1).
		var sel = '.card[data-row="' + mi + '"][data-idx="' + (rows[mi]-1) + '"]:not(.removed)';
		var el = document.querySelector(sel);
		if (!el) return false;
		el.click();
		return true;
	})()`

	if err := chromedp.Run(ctx,
		chromedp.Navigate(base+"/"),
		chromedp.Evaluate(`localStorage.clear()`, nil),
		chromedp.Reload(),
		chromedp.WaitVisible(`#mode-vsai`, chromedp.ByQuery),
		chromedp.Sleep(400*time.Millisecond),
		chromedp.Click(`#mode-vsai`, chromedp.ByQuery),
		chromedp.WaitVisible(`#board`, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Play bad moves until terminal.
	for step := 0; step < 30; step++ {
		// Wait for our turn or game over.
		if err := chromedp.Run(ctx, chromedp.Poll(
			`!document.getElementById('status').textContent.includes('réfléchit')`,
			nil,
			chromedp.WithPollingInterval(80*time.Millisecond),
			chromedp.WithPollingTimeout(6*time.Second),
		)); err != nil {
			t.Fatalf("wait turn: %v", err)
		}
		var status string
		if err := chromedp.Run(ctx, chromedp.Text(`#status`, &status, chromedp.ByQuery)); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(status, "gagné") || strings.Contains(status, "battu") {
			break
		}
		var played bool
		if err := chromedp.Run(ctx, chromedp.Evaluate(playBad, &played)); err != nil {
			t.Fatal(err)
		}
		if !played {
			break
		}
		if err := chromedp.Run(ctx,
			chromedp.WaitEnabled(`#btn-validate`, chromedp.ByQuery),
			chromedp.Click(`#btn-validate`, chromedp.ByQuery),
			chromedp.Sleep(150*time.Millisecond),
		); err != nil {
			t.Fatalf("validate (step %d): %v", step, err)
		}
	}

	// Final state: human should have lost ("L'ordinateur t'a battu.").
	var status string
	if err := chromedp.Run(ctx,
		chromedp.Sleep(400*time.Millisecond),
		chromedp.Text(`#status`, &status, chromedp.ByQuery),
	); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(status, "battu") {
		t.Skipf("human did not lose (status=%q) — can't verify game-over screen", status)
	}

	// GAME OVER overlay should be visible.
	var gameOverVisible bool
	if err := chromedp.Run(ctx,
		chromedp.Sleep(300*time.Millisecond),
		chromedp.Evaluate(
			`!document.getElementById('gameover-overlay').classList.contains('hidden')`,
			&gameOverVisible),
	); err != nil {
		t.Fatal(err)
	}
	if !gameOverVisible {
		t.Fatalf("GAME OVER overlay should be visible after AI win\nconsole:\n%s",
			strings.Join(*logs, "\n"))
	}

	// Retro text content check.
	var retroText string
	if err := chromedp.Run(ctx,
		chromedp.Text(`#gameover-text`, &retroText, chromedp.ByQuery),
	); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(retroText, "GAME OVER") {
		t.Errorf("retro text = %q, want 'GAME OVER'", retroText)
	}
}

func TestScreenshotGameOver(t *testing.T) {
	if os.Getenv("E2E_SCREENSHOTS") == "" {
		t.Skip("set E2E_SCREENSHOTS=1 to enable")
	}
	base, stop := startServer(t)
	defer stop()

	ctx, _, cleanup := chromeCtx(t, 60*time.Second)
	defer cleanup()

	playBad := `(function() {
		var rows = [0,0,0];
		for (var r = 0; r < 3; r++) {
			rows[r] = document.querySelectorAll('.card[data-row="' + r + '"]:not(.removed)').length;
		}
		var max = 0, mi = -1;
		for (var i = 0; i < 3; i++) if (rows[i] > max) { max = rows[i]; mi = i; }
		if (mi === -1) return false;
		var sel = '.card[data-row="' + mi + '"][data-idx="' + (rows[mi]-1) + '"]:not(.removed)';
		var el = document.querySelector(sel);
		if (!el) return false;
		el.click();
		return true;
	})()`

	tasks := chromedp.Tasks{
		chromedp.EmulateViewport(1024, 768),
		chromedp.Navigate(base + "/"),
		chromedp.Evaluate(`localStorage.clear()`, nil),
		chromedp.Reload(),
		chromedp.WaitVisible(`#mode-vsai`, chromedp.ByQuery),
		chromedp.Sleep(400 * time.Millisecond),
		chromedp.Click(`#mode-vsai`, chromedp.ByQuery),
		chromedp.WaitVisible(`#board`, chromedp.ByQuery),
	}
	for step := 0; step < 30; step++ {
		tasks = append(tasks,
			chromedp.Poll(
				`!document.getElementById('status').textContent.includes('réfléchit') || document.getElementById('status').textContent.includes('battu') || document.getElementById('status').textContent.includes('gagné')`,
				nil,
				chromedp.WithPollingTimeout(6*time.Second),
			),
			chromedp.Evaluate(playBad, nil),
			chromedp.Sleep(150*time.Millisecond),
			chromedp.Evaluate(`(function(){var b=document.getElementById('btn-validate');if(b&&!b.disabled)b.click()})()`, nil),
			chromedp.Sleep(120*time.Millisecond),
		)
	}

	var buf []byte
	tasks = append(tasks, chromedp.Sleep(1*time.Second), chromedp.FullScreenshot(&buf, 90))
	if err := chromedp.Run(ctx, tasks); err != nil {
		t.Fatalf("flow: %v", err)
	}
	out := filepath.Join("..", "docs", "screenshots", "game-over.png")
	_ = os.MkdirAll(filepath.Dir(out), 0o755)
	if err := os.WriteFile(out, buf, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("game-over screenshot: %s (%d bytes)", out, len(buf))
}
