package nim

import (
	"testing"
)

func TestNewBoard(t *testing.T) {
	b := NewBoard()
	if b.Rows != [3]int{7, 5, 3} {
		t.Fatalf("expected 7-5-3, got %v", b.Rows)
	}
	if b.IsTerminal() {
		t.Fatal("fresh board should not be terminal")
	}
}

func TestNimSum(t *testing.T) {
	cases := []struct {
		rows [3]int
		want int
	}{
		{[3]int{7, 5, 3}, 7 ^ 5 ^ 3},
		{[3]int{0, 0, 0}, 0},
		{[3]int{1, 1, 0}, 0},
		{[3]int{2, 2, 0}, 0},
		{[3]int{4, 2, 1}, 7},
	}
	for _, c := range cases {
		b := Board{Rows: c.rows}
		if got := b.NimSum(); got != c.want {
			t.Errorf("NimSum(%v) = %d, want %d", c.rows, got, c.want)
		}
	}
}

func TestLegalMoves(t *testing.T) {
	b := Board{Rows: [3]int{2, 0, 1}}
	moves := b.LegalMoves()
	// row 0: 1,2 (2 moves) ; row 1: none ; row 2: 1 (1 move) → 3 total
	if len(moves) != 3 {
		t.Fatalf("expected 3 legal moves, got %d: %v", len(moves), moves)
	}
}

func TestLegalMovesTerminal(t *testing.T) {
	b := Board{Rows: [3]int{0, 0, 0}}
	if moves := b.LegalMoves(); moves != nil {
		t.Fatalf("expected no moves on terminal, got %v", moves)
	}
}

func TestApplyValid(t *testing.T) {
	b := NewBoard()
	nb, err := b.Apply(Move{Row: 0, Count: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nb.Rows != [3]int{4, 5, 3} {
		t.Fatalf("expected 4-5-3 after removing 3 from row 0, got %v", nb.Rows)
	}
	// Receiver must not be mutated.
	if b.Rows != [3]int{7, 5, 3} {
		t.Fatalf("Apply mutated receiver: %v", b.Rows)
	}
}

func TestApplyIllegal(t *testing.T) {
	b := NewBoard()
	bad := []Move{
		{Row: -1, Count: 1},
		{Row: 3, Count: 1},
		{Row: 0, Count: 0},
		{Row: 0, Count: 8}, // > 7
		{Row: 2, Count: 4}, // > 3
	}
	for _, m := range bad {
		if _, err := b.Apply(m); err == nil {
			t.Errorf("expected error for %+v", m)
		}
	}
}

func TestIsTerminal(t *testing.T) {
	if !(Board{Rows: [3]int{0, 0, 0}}).IsTerminal() {
		t.Fatal("0-0-0 should be terminal")
	}
	if (Board{Rows: [3]int{0, 1, 0}}).IsTerminal() {
		t.Fatal("0-1-0 should not be terminal")
	}
}

func TestPlayerOther(t *testing.T) {
	if P1.Other() != P2 || P2.Other() != P1 {
		t.Fatal("Other should toggle players")
	}
}
