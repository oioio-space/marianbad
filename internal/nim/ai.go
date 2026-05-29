package nim

import "math/rand/v2"

// ChooseMove returns the AI's move for the given board.
//
// The strategy implements the classical misère-Nim optimal play:
//
//  1. Endgame (no row has ≥ 2 matches): only forced moves remain — take 1
//     from any non-empty row. With perfect play, whoever entered this endgame
//     with an odd number of 1-rows loses.
//
//  2. Exactly one row has ≥ 2 matches: this is the misère pivot. The AI
//     reduces that big row to 0 or 1 so that the resulting count of 1-rows
//     is ODD (which is a losing position for the opponent in misère).
//
//  3. Two or more rows have ≥ 2 matches: standard Nim — if NimSum != 0,
//     play the move that zeroes it. If NimSum == 0 the position is losing
//     against perfect play; fall back to the "tricky" heuristic that
//     maximizes the chance the human opponent miscalculates.
func ChooseMove(b Board) Move {
	if b.IsTerminal() {
		// Defensive: caller shouldn't ask for a move on a terminal board.
		return Move{}
	}

	r2 := b.rowsWithAtLeastTwo()

	switch {
	case r2 == 0:
		return endgameMove(b)
	case r2 == 1:
		return misèrePivotMove(b)
	default:
		if b.NimSum() != 0 {
			return winningNimMove(b)
		}
		return trickyFallback(b)
	}
}

// endgameMove: every non-empty row has exactly 1 match. Take 1 from the
// lowest-index non-empty row (deterministic, prolongs the game).
func endgameMove(b Board) Move {
	for r := 0; r < 3; r++ {
		if b.Rows[r] > 0 {
			return Move{Row: r, Count: 1}
		}
	}
	return Move{}
}

// misèrePivotMove handles the case where exactly one row has ≥ 2 matches.
// We collapse that row to 0 or 1 so the opponent faces an odd number of 1s.
func misèrePivotMove(b Board) Move {
	bigRow := -1
	for r := 0; r < 3; r++ {
		if b.Rows[r] >= 2 {
			bigRow = r
			break
		}
	}
	onesBefore := b.rowsWithOne()

	// Leaving 1 means onesAfter = onesBefore + 1.
	// Leaving 0 means onesAfter = onesBefore.
	// We want onesAfter to be ODD.
	var leave int
	if onesBefore%2 == 0 {
		leave = 1 // makes onesAfter odd
	} else {
		leave = 0 // keeps onesAfter odd
	}
	return Move{Row: bigRow, Count: b.Rows[bigRow] - leave}
}

// winningNimMove: standard Nim — find a row r such that Rows[r] XOR nimSum
// is strictly less than Rows[r]; play to bring that row down to that value.
func winningNimMove(b Board) Move {
	ns := b.NimSum()
	for r := 0; r < 3; r++ {
		target := b.Rows[r] ^ ns
		if target < b.Rows[r] {
			return Move{Row: r, Count: b.Rows[r] - target}
		}
	}
	// Unreachable when NimSum != 0; fall back defensively.
	return trickyFallback(b)
}

// trickyFallback: AI is in a theoretically losing position. Play a move
// that maximizes the chance the human miscalculates. Heuristic:
//
//   - keep as many "rich" rows (≥ 2 matches) on the board as possible
//   - keep the total non-empty row count high
//   - among ties, take the smallest count (prolong the game)
//
// Score formula: rowsWithAtLeastTwo*100 + nonEmptyRows*10 - count.
func trickyFallback(b Board) Move {
	best := Move{Row: -1}
	bestScore := -1 << 30

	// Random ordering of legal moves so equal-scoring choices vary.
	moves := b.LegalMoves()
	rand.Shuffle(len(moves), func(i, j int) { moves[i], moves[j] = moves[j], moves[i] })

	for _, m := range moves {
		nb, err := b.Apply(m)
		if err != nil {
			continue
		}
		score := scoreForTricky(nb, m.Count)
		if score > bestScore {
			bestScore = score
			best = m
		}
	}
	if best.Row == -1 {
		// Shouldn't happen on a non-terminal board, but stay safe.
		return endgameMove(b)
	}
	return best
}

func scoreForTricky(after Board, count int) int {
	rich := after.rowsWithAtLeastTwo()
	nonEmpty := 0
	for _, r := range after.Rows {
		if r > 0 {
			nonEmpty++
		}
	}
	return rich*100 + nonEmpty*10 - count
}
