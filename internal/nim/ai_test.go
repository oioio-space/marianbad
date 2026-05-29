package nim

import (
	"math/rand/v2"
	"testing"
)

// applyOrFail panics in test mode if the move is illegal.
func applyOrFail(t *testing.T, b Board, m Move) Board {
	t.Helper()
	nb, err := b.Apply(m)
	if err != nil {
		t.Fatalf("illegal move %+v on %v: %v", m, b.Rows, err)
	}
	return nb
}

func TestChooseMoveAlwaysLegal(t *testing.T) {
	// 1000 random non-terminal positions.
	for i := 0; i < 1000; i++ {
		b := Board{Rows: [3]int{rand.IntN(8), rand.IntN(8), rand.IntN(8)}}
		if b.IsTerminal() {
			continue
		}
		m := ChooseMove(b)
		if _, err := b.Apply(m); err != nil {
			t.Fatalf("ChooseMove returned illegal move %+v on %v", m, b.Rows)
		}
	}
}

func TestChooseMoveStandardNim(t *testing.T) {
	// Position with ≥ 2 rows of size ≥ 2 and NimSum != 0 → optimal move
	// must bring NimSum to 0.
	cases := [][3]int{
		{7, 5, 3},
		{4, 5, 3},
		{6, 4, 2},
	}
	for _, rows := range cases {
		b := Board{Rows: rows}
		if b.rowsWithAtLeastTwo() < 2 {
			t.Skipf("position %v not in standard-Nim regime", rows)
		}
		if b.NimSum() == 0 {
			continue
		}
		m := ChooseMove(b)
		nb := applyOrFail(t, b, m)
		if nb.NimSum() != 0 {
			t.Errorf("ChooseMove(%v) → %+v left NimSum=%d, want 0", rows, m, nb.NimSum())
		}
	}
}

func TestChooseMoveMisèrePivot(t *testing.T) {
	// Exactly one row ≥ 2 — AI must leave an odd number of 1-rows.
	cases := [][3]int{
		{5, 1, 1}, // R1 before = 2 (even) → leave 1 in big row → onesAfter = 3 (odd)
		{5, 0, 1}, // R1 before = 1 (odd) → leave 0 in big row → onesAfter = 1 (odd)
		{4, 0, 0}, // R1 before = 0 (even) → leave 1 in big row → onesAfter = 1 (odd)
		{6, 1, 0}, // R1 before = 1 (odd) → leave 0 → onesAfter = 1
	}
	for _, rows := range cases {
		b := Board{Rows: rows}
		m := ChooseMove(b)
		nb := applyOrFail(t, b, m)
		onesAfter := nb.rowsWithOne()
		if onesAfter%2 != 1 {
			t.Errorf("ChooseMove(%v) → %+v left %d 1-rows, want odd",
				rows, m, onesAfter)
		}
		// And no row ≥ 2 remains.
		if nb.rowsWithAtLeastTwo() != 0 {
			t.Errorf("ChooseMove(%v) → %+v left rich rows: %v",
				rows, m, nb.Rows)
		}
	}
}

func TestChooseMoveEndgame(t *testing.T) {
	cases := [][3]int{
		{1, 1, 1},
		{1, 0, 1},
		{0, 0, 1},
		{1, 1, 0},
	}
	for _, rows := range cases {
		b := Board{Rows: rows}
		m := ChooseMove(b)
		if m.Count != 1 {
			t.Errorf("endgame ChooseMove(%v) → %+v, want Count=1", rows, m)
		}
		if _, err := b.Apply(m); err != nil {
			t.Errorf("illegal: %v", err)
		}
	}
}

func TestChooseMoveTricky(t *testing.T) {
	// Forced losing position with two rich rows: NimSum=0, e.g. {3,3,0}.
	// Heuristic should keep things rich; here it has no choice but to break
	// the symmetry. Just verify the move is legal and the game continues.
	b := Board{Rows: [3]int{3, 3, 0}}
	m := ChooseMove(b)
	nb := applyOrFail(t, b, m)
	if nb.IsTerminal() {
		t.Fatal("tricky fallback should not immediately end the game from {3,3,0}")
	}
}

// Game terminates within at most total-matches moves.
func TestGameTerminates(t *testing.T) {
	for trial := 0; trial < 200; trial++ {
		b := NewBoard()
		steps := 0
		maxSteps := b.Rows[0] + b.Rows[1] + b.Rows[2]
		for !b.IsTerminal() {
			b = applyOrFail(t, b, ChooseMove(b))
			steps++
			if steps > maxSteps {
				t.Fatalf("game did not terminate in %d steps", maxSteps)
			}
		}
	}
}

// AI versus a player that picks a uniformly random legal move.
// With misère-optimal play, the AI should dominate.
func TestAIBeatsRandom(t *testing.T) {
	const games = 1000
	aiWins := 0
	for g := 0; g < games; g++ {
		b := NewBoard()
		// Alternate starter: half the games AI starts, half random starts.
		aiToPlay := g%2 == 0
		// Track who took the last match — that player LOSES (misère).
		var lastMover string
		for !b.IsTerminal() {
			var m Move
			if aiToPlay {
				m = ChooseMove(b)
				lastMover = "ai"
			} else {
				moves := b.LegalMoves()
				m = moves[rand.IntN(len(moves))]
				lastMover = "rand"
			}
			b = applyOrFail(t, b, m)
			aiToPlay = !aiToPlay
		}
		// Last mover loses; opponent wins.
		if lastMover == "rand" {
			aiWins++
		}
	}
	// Empirically the AI wins well over 95% — set a conservative bar.
	if aiWins < games*90/100 {
		t.Errorf("AI won only %d/%d vs random, want ≥ 90%%", aiWins, games)
	}
}

// AI vs AI: with misère-perfect play, the player who faces a position with
// (rowsWithAtLeastTwo >= 2 AND nimSum == 0) loses. Starting from {7,5,3},
// nimSum = 7^5^3 = 1 ≠ 0 → first player wins.
func TestAIvsAIStarterWinsOn753(t *testing.T) {
	b := NewBoard()
	if b.NimSum() == 0 {
		t.Skip("starting position is theoretically losing for first player")
	}
	firstToPlay := true
	var lastMover string
	for !b.IsTerminal() {
		b = applyOrFail(t, b, ChooseMove(b))
		if firstToPlay {
			lastMover = "first"
		} else {
			lastMover = "second"
		}
		firstToPlay = !firstToPlay
	}
	// Last to take loses, so first wins iff lastMover == "second".
	if lastMover != "second" {
		t.Errorf("expected first player (mover from winning 7-5-3) to win, last mover was %q", lastMover)
	}
}
