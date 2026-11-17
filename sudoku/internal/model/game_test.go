package model_test

import (
	"errors"
	"testing"
	"time"

	"jiostar/sudoku/internal/errs"
	"jiostar/sudoku/internal/model"
	"jiostar/sudoku/internal/validator"
)

var solved4 = [][]int{
	{1, 2, 3, 4},
	{3, 4, 1, 2},
	{2, 1, 4, 3},
	{4, 3, 2, 1},
}

// newGame blanks the given cells of solved4 and starts a game.
func newGame(t *testing.T, blanks ...model.Position) *model.Game {
	t.Helper()
	grid := model.CopyGrid(solved4)
	for _, p := range blanks {
		grid[p.Row][p.Col] = 0
	}
	b, err := model.NewBoardFromGrid(grid)
	if err != nil {
		t.Fatal(err)
	}
	return model.NewGame("g", "u", model.Easy, b, validator.NewStandardValidator(), time.Now())
}

func mustMove(t *testing.T, g *model.Game, p model.Position, v int) (model.MoveResult, model.GameState) {
	t.Helper()
	res, st, err := g.ApplyMove(p, v)
	if err != nil {
		t.Fatalf("move %v=%d: %v", p, v, err)
	}
	if err := g.Verify(); err != nil {
		t.Fatalf("after move %v=%d: %v", p, v, err)
	}
	return res, st
}

func TestConflictIsPlacedAndFlagged(t *testing.T) {
	a := model.Position{Row: 0, Col: 0}
	g := newGame(t, a)
	res, st := mustMove(t, g, a, 2) // 2 already in row 0 (fixed)
	if res.Valid || len(res.Conflicts) == 0 {
		t.Fatalf("want invalid move, got %+v", res)
	}
	if st.Board.Grid[0][0] != 2 || !st.Board.Invalid[0][0] {
		t.Fatal("conflicting value should be placed and flagged")
	}
	if st.Board.Invalid[0][1] {
		t.Fatal("fixed cells must never be flagged")
	}
}

func TestOverwriteAndClearUnflagPartners(t *testing.T) {
	a, b := model.Position{Row: 0, Col: 0}, model.Position{Row: 0, Col: 1}
	g := newGame(t, a, b)

	mustMove(t, g, a, 3) // 3 clashes with fixed (0,2); and later with b
	_, st := mustMove(t, g, b, 3)
	if !st.Board.Invalid[0][0] || !st.Board.Invalid[0][1] {
		t.Fatal("both player cells should be flagged")
	}

	// Overwrite b with its correct value: b unflags, a stays flagged (still clashes with fixed (0,2)).
	_, st = mustMove(t, g, b, 2)
	if st.Board.Invalid[0][1] || !st.Board.Invalid[0][0] {
		t.Fatalf("after fixing b: invalid a=%v b=%v", st.Board.Invalid[0][0], st.Board.Invalid[0][1])
	}

	// Clearing a removes its flag.
	res, st := mustMove(t, g, a, 0)
	if !res.Valid || st.Board.Invalid[0][0] || st.Board.Grid[0][0] != 0 {
		t.Fatal("clear should succeed and unflag")
	}
}

func TestPartnerUnflaggedWhenConflictSourceIsCleared(t *testing.T) {
	a, b := model.Position{Row: 0, Col: 0}, model.Position{Row: 1, Col: 1}
	g := newGame(t, a, b) // same box; correct values are 1 and 4
	mustMove(t, g, a, 4)  // clashes with fixed 4 at (0,3) and later with b
	mustMove(t, g, b, 4)
	_, st := mustMove(t, g, a, 0)
	if st.Board.Invalid[1][1] {
		t.Fatal("b should be unflagged once a is cleared")
	}
}

func TestCompletion(t *testing.T) {
	a, b := model.Position{Row: 0, Col: 0}, model.Position{Row: 0, Col: 1}
	g := newGame(t, a, b)

	mustMove(t, g, a, 2)
	_, st := mustMove(t, g, b, 1) // full board, but row 0 is 2,1,3,4 → column clashes
	if st.Status != model.InProgress {
		t.Fatal("a full board with conflicts must stay in progress")
	}
	mustMove(t, g, a, 1)
	_, st = mustMove(t, g, b, 2)
	if st.Status != model.Completed {
		t.Fatalf("status=%s, want COMPLETED", st.Status)
	}
	if _, _, err := g.ApplyMove(a, 0); !errors.Is(err, errs.ErrGameCompleted) {
		t.Fatalf("move after completion → %v, want ErrGameCompleted", err)
	}
}

func TestRejectedMoveChangesNothing(t *testing.T) {
	g := newGame(t, model.Position{Row: 0, Col: 0})
	before := g.State()
	for _, tc := range []struct {
		p   model.Position
		v   int
		err error
	}{
		{model.Position{Row: 0, Col: 1}, 1, errs.ErrFixedCell},
		{model.Position{Row: 0, Col: 0}, 5, errs.ErrInvalidValue},
		{model.Position{Row: 4, Col: 0}, 1, errs.ErrOutOfBounds},
	} {
		if _, _, err := g.ApplyMove(tc.p, tc.v); !errors.Is(err, tc.err) {
			t.Errorf("%v=%d → %v, want %v", tc.p, tc.v, err, tc.err)
		}
	}
	after := g.State()
	if after.MoveCount != before.MoveCount || after.Board.Grid[0][0] != 0 {
		t.Fatal("rejected moves must not change the game")
	}
}
