// Package nim implements the Marienbad (Nim, misère variant) game engine.
//
// Rules: 3 rows of 7-5-3 matches. Players alternate; on each turn a player
// removes 1..N matches from a single row. The player who takes the LAST
// match loses (misère).
package nim

import "errors"

// InitialRows is the fixed starting layout: 7 - 5 - 3.
var InitialRows = [3]int{7, 5, 3}

// Board is the immutable game position. Apply returns a new Board.
type Board struct {
	Rows [3]int
}

// Move describes removing Count matches from row Row.
type Move struct {
	Row   int
	Count int
}

// Player identifies who plays. In VsAI mode P1 is the human and P2 the AI.
// In TwoPlayer mode P1 and P2 are both humans.
type Player int

const (
	P1 Player = iota
	P2
)

// Mode is the high-level game mode.
type Mode int

const (
	VsAI Mode = iota
	TwoPlayer
)

// Other returns the opposite player.
func (p Player) Other() Player {
	if p == P1 {
		return P2
	}
	return P1
}

// NewBoard returns a fresh 7-5-3 board.
func NewBoard() Board {
	return Board{Rows: InitialRows}
}

// IsTerminal reports whether the game is over (no matches left).
func (b Board) IsTerminal() bool {
	return b.Rows[0]+b.Rows[1]+b.Rows[2] == 0
}

// NimSum is the XOR of all row sizes — the classical Nim invariant.
func (b Board) NimSum() int {
	return b.Rows[0] ^ b.Rows[1] ^ b.Rows[2]
}

// LegalMoves enumerates every valid move from this position.
func (b Board) LegalMoves() []Move {
	if b.IsTerminal() {
		return nil
	}
	out := make([]Move, 0, b.Rows[0]+b.Rows[1]+b.Rows[2])
	for r := 0; r < 3; r++ {
		for c := 1; c <= b.Rows[r]; c++ {
			out = append(out, Move{Row: r, Count: c})
		}
	}
	return out
}

// ErrIllegalMove is returned by Apply when m violates the rules.
var ErrIllegalMove = errors.New("nim: illegal move")

// Apply returns the board after playing m, or an error if m is illegal.
// The receiver is not mutated.
func (b Board) Apply(m Move) (Board, error) {
	if m.Row < 0 || m.Row >= 3 {
		return b, ErrIllegalMove
	}
	if m.Count < 1 || m.Count > b.Rows[m.Row] {
		return b, ErrIllegalMove
	}
	nb := b
	nb.Rows[m.Row] -= m.Count
	return nb, nil
}

// rowsWithAtLeastTwo counts rows containing at least 2 matches.
// When this drops to 0 we are in the misère endgame.
func (b Board) rowsWithAtLeastTwo() int {
	n := 0
	for _, r := range b.Rows {
		if r >= 2 {
			n++
		}
	}
	return n
}

// rowsWithOne counts rows containing exactly 1 match.
func (b Board) rowsWithOne() int {
	n := 0
	for _, r := range b.Rows {
		if r == 1 {
			n++
		}
	}
	return n
}
