//go:build js && wasm

package main

import (
	"fmt"
	"math/rand/v2"
	"syscall/js"

	"github.com/marianbad/internal/nim"
)

const aiThinkMs = 650

type Controller struct {
	mode      nim.Mode
	modeSet   bool
	board     nim.Board
	toPlay    nim.Player
	starter   nim.Player // who started THIS game
	winner    *nim.Player
	score     Score

	selRow  int // -1 if no selection
	selFrom int // first selected card index in selRow (selection is selFrom..end)

	handlers []js.Func
	aiTimer  js.Value
}

func New() *Controller {
	return &Controller{
		score:  loadScore(),
		selRow: -1,
	}
}

// Mount wires up the persistent (cross-game) DOM elements: mode buttons,
// restart, validate, cancel, reset-score, change-mode.
func (c *Controller) Mount() {
	c.bindGlobal()
	c.renderModeSelector()
}

func (c *Controller) bindGlobal() {
	c.addHandler(byID("mode-vsai"), "click", func(js.Value, []js.Value) {
		c.setMode(nim.VsAI)
	})
	c.addHandler(byID("mode-2p"), "click", func(js.Value, []js.Value) {
		c.setMode(nim.TwoPlayer)
	})
	c.addHandler(byID("btn-restart"), "click", func(js.Value, []js.Value) {
		c.startNewGame(false)
	})
	c.addHandler(byID("btn-change-mode"), "click", func(js.Value, []js.Value) {
		c.modeSet = false
		clearTimeout(c.aiTimer)
		c.aiTimer = js.Undefined()
		c.renderModeSelector()
	})
	c.addHandler(byID("btn-validate"), "click", func(js.Value, []js.Value) {
		c.onValidate()
	})
	c.addHandler(byID("btn-cancel"), "click", func(js.Value, []js.Value) {
		c.clearSelection()
		c.renderSelection()
		c.updateActionButtons()
	})
	c.addHandler(byID("btn-reset-score"), "click", func(js.Value, []js.Value) {
		if window.Call("confirm", "Réinitialiser le score de ce mode ?").Bool() {
			c.score.resetMode(c.mode)
			saveScore(c.score)
			c.renderScore()
		}
	})

	for _, card := range querySelectorAll(".card") {
		row := card.Get("dataset").Get("row").String()
		idx := card.Get("dataset").Get("idx").String()
		var r, i int
		fmt.Sscanf(row, "%d", &r)
		fmt.Sscanf(idx, "%d", &i)
		rr, ii := r, i
		c.addHandler(card, "click", func(js.Value, []js.Value) {
			c.onCardClick(rr, ii)
		})
	}
}

func (c *Controller) addHandler(el js.Value, event string, fn func(this js.Value, args []js.Value)) {
	if el.IsNull() || el.IsUndefined() {
		return
	}
	c.handlers = append(c.handlers, on(el, event, fn))
}

// ----- Screens -------------------------------------------------------------

func (c *Controller) renderModeSelector() {
	setHidden(byID("mode-selector"), false)
	setHidden(byID("game-view"), true)
}

func (c *Controller) renderGameView() {
	setHidden(byID("mode-selector"), true)
	setHidden(byID("game-view"), false)
	c.renderScore()
}

// ----- Game flow -----------------------------------------------------------

func (c *Controller) setMode(m nim.Mode) {
	c.mode = m
	c.modeSet = true
	c.startNewGame(true)
	c.renderGameView()
}

// startNewGame resets the board. If firstOfMode is true, the starter is
// drawn fresh; otherwise we alternate the starter to fairness.
func (c *Controller) startNewGame(firstOfMode bool) {
	clearTimeout(c.aiTimer)
	c.aiTimer = js.Undefined()
	// Dismiss any leftover victory modal / fireworks / game-over from the
	// previous game.
	invokeJS("marianbadHideVictory")
	invokeJS("marianbadHideGameOver")

	c.board = nim.NewBoard()
	c.winner = nil
	c.clearSelection()

	if firstOfMode {
		if rand.IntN(2) == 0 {
			c.starter = nim.P1
		} else {
			c.starter = nim.P2
		}
	} else {
		c.starter = c.starter.Other()
	}
	c.toPlay = c.starter

	c.renderBoardReset()
	c.renderSelection()
	c.updateActionButtons()
	c.renderStatus()

	if c.mode == nim.VsAI && c.toPlay == nim.P2 {
		c.scheduleAITurn()
	}
}

func (c *Controller) clearSelection() {
	c.selRow = -1
	c.selFrom = 0
}

func (c *Controller) onCardClick(row, idx int) {
	if c.winner != nil {
		return
	}
	if c.mode == nim.VsAI && c.toPlay == nim.P2 {
		return // AI's turn
	}
	// Card must be present (not removed) and the row must still have it.
	if idx >= c.board.Rows[row] {
		return
	}

	if c.selRow == -1 || c.selRow == row {
		c.selRow = row
		c.selFrom = idx
	} else {
		// Different row while a selection exists: shake feedback.
		c.shakeRow(row)
		return
	}
	c.renderSelection()
	c.updateActionButtons()
}

func (c *Controller) onValidate() {
	if c.selRow == -1 {
		return
	}
	count := c.board.Rows[c.selRow] - c.selFrom
	if count < 1 {
		return
	}
	move := nim.Move{Row: c.selRow, Count: count}
	nb, err := c.board.Apply(move)
	if err != nil {
		window.Get("console").Call("error", js.ValueOf("illegal move"))
		return
	}
	c.animateRemoval(move)
	c.board = nb
	c.clearSelection()
	c.renderSelection()

	if c.board.IsTerminal() {
		// Mover took the last match → loses (misère).
		loser := c.toPlay
		winner := loser.Other()
		c.winner = &winner
		c.score.recordWin(c.mode, winner)
		saveScore(c.score)
		c.renderScore()
		c.renderStatus()
		c.updateActionButtons()
		c.celebrateIfPlayerWon()
		return
	}

	c.toPlay = c.toPlay.Other()
	c.renderStatus()
	c.updateActionButtons()

	if c.mode == nim.VsAI && c.toPlay == nim.P2 {
		c.scheduleAITurn()
	}
}

func (c *Controller) scheduleAITurn() {
	addClass(byID("board"), "ai-thinking")
	c.aiTimer = setTimeoutMS(aiThinkMs, func() {
		c.aiTimer = js.Undefined()
		removeClass(byID("board"), "ai-thinking")
		if c.winner != nil || !c.modeSet {
			return
		}
		move := nim.ChooseMove(c.board)
		c.animateRemoval(move)
		nb, err := c.board.Apply(move)
		if err != nil {
			window.Get("console").Call("error", js.ValueOf("AI illegal move"))
			return
		}
		c.board = nb
		if c.board.IsTerminal() {
			loser := c.toPlay
			winner := loser.Other()
			c.winner = &winner
			c.score.recordWin(c.mode, winner)
			saveScore(c.score)
			c.renderScore()
			c.renderStatus()
			c.updateActionButtons()
			c.celebrateIfPlayerWon()
			return
		}
		c.toPlay = c.toPlay.Other()
		c.renderStatus()
		c.updateActionButtons()
	})
}

// ----- Rendering -----------------------------------------------------------

// renderBoardReset un-hides every card slot and clears state classes.
func (c *Controller) renderBoardReset() {
	for _, card := range querySelectorAll(".card") {
		removeClass(card, "removed")
		removeClass(card, "selected")
		removeClass(card, "shake")
		setDisabled(card, false)
	}
}

func (c *Controller) renderSelection() {
	for _, card := range querySelectorAll(".card") {
		removeClass(card, "selected")
	}
	if c.selRow == -1 {
		return
	}
	for i := c.selFrom; i < c.board.Rows[c.selRow]; i++ {
		card := c.cardEl(c.selRow, i)
		if !card.IsNull() && !card.IsUndefined() {
			addClass(card, "selected")
		}
	}
}

func (c *Controller) animateRemoval(m nim.Move) {
	from := c.board.Rows[m.Row] - m.Count
	for i := from; i < c.board.Rows[m.Row]; i++ {
		card := c.cardEl(m.Row, i)
		if !card.IsNull() && !card.IsUndefined() {
			removeClass(card, "selected")
			addClass(card, "removed")
		}
	}
}

func (c *Controller) shakeRow(row int) {
	rowEl := doc.Call("querySelector", fmt.Sprintf(".row[data-row=\"%d\"]", row))
	if rowEl.IsNull() || rowEl.IsUndefined() {
		return
	}
	addClass(rowEl, "shake")
	setTimeoutMS(350, func() { removeClass(rowEl, "shake") })
}

func (c *Controller) cardEl(row, idx int) js.Value {
	sel := fmt.Sprintf(".card[data-row=\"%d\"][data-idx=\"%d\"]", row, idx)
	return doc.Call("querySelector", sel)
}

func (c *Controller) updateActionButtons() {
	playable := c.winner == nil &&
		!(c.mode == nim.VsAI && c.toPlay == nim.P2)
	canValidate := playable && c.selRow != -1
	canCancel := playable && c.selRow != -1
	setDisabled(byID("btn-validate"), !canValidate)
	setDisabled(byID("btn-cancel"), !canCancel)
}

func (c *Controller) renderStatus() {
	var msg string
	if c.winner != nil {
		msg = c.winnerMessage()
	} else {
		msg = c.turnMessage()
	}
	setText(byID("status"), msg)
}

func (c *Controller) winnerMessage() string {
	w := *c.winner
	switch c.mode {
	case nim.VsAI:
		if w == nim.P1 {
			return "Bravo, tu as gagné !"
		}
		return "L'ordinateur t'a battu."
	case nim.TwoPlayer:
		if w == nim.P1 {
			return "Joueur 1 a gagné !"
		}
		return "Joueur 2 a gagné !"
	}
	return ""
}

// announceOutcome shows the appropriate end-of-game overlay:
//   - human victory (VsAI human win, or any 2-player win): fireworks + modal
//   - human defeat against the AI: retro "GAME OVER" overlay
func (c *Controller) announceOutcome() {
	if c.winner == nil {
		return
	}
	w := *c.winner
	switch c.mode {
	case nim.VsAI:
		if w == nim.P1 {
			invokeJS("marianbadCelebrate", "Victoire !", "Tu as battu l'ordinateur.")
		} else {
			invokeJS("marianbadGameOver", "L'ordinateur t'a battu.")
		}
	case nim.TwoPlayer:
		msg := "Joueur 1 remporte la partie."
		if w == nim.P2 {
			msg = "Joueur 2 remporte la partie."
		}
		invokeJS("marianbadCelebrate", "Victoire !", msg)
	}
}

func invokeJS(name string, args ...any) {
	fn := window.Get(name)
	if fn.IsUndefined() || fn.IsNull() {
		return
	}
	jsArgs := make([]any, len(args))
	for i, a := range args {
		jsArgs[i] = a
	}
	fn.Invoke(jsArgs...)
}

// celebrateIfPlayerWon is kept as a thin wrapper for backwards-compat with
// existing call sites; new code should call announceOutcome directly.
func (c *Controller) celebrateIfPlayerWon() { c.announceOutcome() }

func (c *Controller) turnMessage() string {
	switch c.mode {
	case nim.VsAI:
		if c.toPlay == nim.P1 {
			return "À toi de jouer."
		}
		return "L'ordinateur réfléchit…"
	case nim.TwoPlayer:
		if c.toPlay == nim.P1 {
			return "Joueur 1, à toi."
		}
		return "Joueur 2, à toi."
	}
	return ""
}

func (c *Controller) renderScore() {
	var line string
	switch c.mode {
	case nim.VsAI:
		line = fmt.Sprintf("Toi : %d  •  Ordi : %d",
			c.score.VsAI.HumanWins, c.score.VsAI.AIWins)
	case nim.TwoPlayer:
		line = fmt.Sprintf("Joueur 1 : %d  •  Joueur 2 : %d",
			c.score.TwoPlayer.P1Wins, c.score.TwoPlayer.P2Wins)
	}
	setText(byID("score-display"), line)
}
