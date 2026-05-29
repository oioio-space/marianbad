package e2e

import (
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// TestFireworksActuallyRender plays a winning 2P game and then samples the
// fireworks canvas a few times to confirm the library actually paints pixels.
// This catches the "canvas exists but is blank" failure mode.
func TestFireworksActuallyRender(t *testing.T) {
	base, stop := startServer(t)
	defer stop()

	ctx, logs, cleanup := chromeCtx(t, 45*time.Second)
	defer cleanup()

	// Get to a winning state via 2P mode (no AI timing flakiness).
	moves := []struct{ row, idx int }{
		{2, 0}, {1, 0},
		{0, 6}, {0, 5}, {0, 4}, {0, 3}, {0, 2}, {0, 1}, {0, 0},
	}

	tasks := chromedp.Tasks{
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
			chromedp.Sleep(120*time.Millisecond),
		)
	}
	if err := chromedp.Run(ctx, tasks); err != nil {
		t.Fatalf("play: %v\nconsole:\n%s", err, strings.Join(*logs, "\n"))
	}

	// Verify the lib was loaded.
	var libPresent bool
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(
			`!!(window.Fireworks && (window.Fireworks.Fireworks || typeof window.Fireworks === 'function'))`,
			&libPresent),
	); err != nil {
		t.Fatal(err)
	}
	if !libPresent {
		t.Fatal("window.Fireworks not exposed by UMD bundle")
	}

	// Verify modal is visible.
	var modalVisible bool
	if err := chromedp.Run(ctx,
		chromedp.Sleep(200*time.Millisecond),
		chromedp.Evaluate(
			`!document.getElementById('victory-modal').classList.contains('hidden')`,
			&modalVisible),
	); err != nil {
		t.Fatal(err)
	}
	if !modalVisible {
		t.Fatal("victory modal did not appear")
	}

	// Verify the overlay is visible.
	var overlayDisplay string
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(
			`getComputedStyle(document.getElementById('fireworks-overlay')).display`,
			&overlayDisplay),
	); err != nil {
		t.Fatal(err)
	}
	if overlayDisplay == "none" {
		t.Fatal("fireworks overlay is display:none after celebration")
	}

	// Wait for animation to actually paint something.
	// Sample the canvas ImageData a few times: as soon as any non-zero alpha
	// pixel appears, the library is genuinely drawing.
	const sampleScript = `
		(function() {
			var canvas = document.querySelector('#fireworks-overlay canvas');
			if (!canvas) return { ok: false, reason: 'no canvas' };
			var w = canvas.width, h = canvas.height;
			if (w === 0 || h === 0) return { ok: false, reason: 'zero size', w: w, h: h };
			var ctx = canvas.getContext('2d');
			var data = ctx.getImageData(0, 0, w, h).data;
			// Count non-zero alpha pixels.
			var lit = 0;
			for (var i = 3; i < data.length; i += 4) {
				if (data[i] > 0) lit++;
			}
			return { ok: lit > 0, lit: lit, w: w, h: h };
		})()
	`

	var result struct {
		OK     bool `json:"ok"`
		Lit    int  `json:"lit"`
		W      int  `json:"w"`
		H      int  `json:"h"`
		Reason string `json:"reason"`
	}

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if err := chromedp.Run(ctx,
			chromedp.Sleep(400*time.Millisecond),
			chromedp.Evaluate(sampleScript, &result),
		); err != nil {
			t.Fatalf("sample: %v", err)
		}
		if result.OK {
			t.Logf("fireworks painting %d pixels on %dx%d", result.Lit, result.W, result.H)
			return
		}
	}
	t.Fatalf("no pixels painted on fireworks canvas after 8s: %+v\nconsole:\n%s",
		result, strings.Join(*logs, "\n"))
}
