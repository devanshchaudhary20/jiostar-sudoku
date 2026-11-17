package validator

import (
	"errors"
	"testing"

	"jiostar/sudoku/internal/errs"
	"jiostar/sudoku/internal/model"
)

func emptyBoard(t *testing.T, size int) *model.Board {
	t.Helper()
	b, err := model.NewBoard(size)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestValidateInputValueRange(t *testing.T) {
	v := NewStandardValidator()
	for _, size := range []int{4, 9, 16} {
		b := emptyBoard(t, size)
		for val := 0; val <= size; val++ {
			if err := v.ValidateInput(b, model.Position{}, val); err != nil {
				t.Errorf("%dx%d: value %d rejected: %v", size, size, val, err)
			}
		}
		for _, val := range []int{-1, size + 1} {
			if err := v.ValidateInput(b, model.Position{}, val); !errors.Is(err, errs.ErrInvalidValue) {
				t.Errorf("%dx%d: value %d → %v, want ErrInvalidValue", size, size, val, err)
			}
		}
	}
}

func TestValidateInputBoundsAndFixed(t *testing.T) {
	v := NewStandardValidator()
	b := emptyBoard(t, 9)
	for _, p := range []model.Position{{Row: -1}, {Col: -1}, {Row: 9}, {Col: 9}} {
		if err := v.ValidateInput(b, p, 1); !errors.Is(err, errs.ErrOutOfBounds) {
			t.Errorf("%v → %v, want ErrOutOfBounds", p, err)
		}
	}
	grid := make([][]int, 9)
	for r := range grid {
		grid[r] = make([]int, 9)
	}
	grid[4][4] = 7
	fb, _ := model.NewBoardFromGrid(grid)
	if err := v.ValidateInput(fb, model.Position{Row: 4, Col: 4}, 0); !errors.Is(err, errs.ErrFixedCell) {
		t.Errorf("clearing a fixed cell → %v, want ErrFixedCell", err)
	}
}

func TestUnitsAtCornersAndEdges(t *testing.T) {
	v := NewStandardValidator()
	b := emptyBoard(t, 9)
	cases := []struct {
		p              model.Position
		boxRow, boxCol int
	}{
		{model.Position{Row: 0, Col: 0}, 0, 0},
		{model.Position{Row: 8, Col: 8}, 6, 6},
		{model.Position{Row: 0, Col: 8}, 0, 6},
		{model.Position{Row: 5, Col: 3}, 3, 3},
	}
	for _, tc := range cases {
		units := v.Units(b, tc.p)
		if len(units) != 3 {
			t.Fatalf("got %d units", len(units))
		}
		for i, q := range units[0] {
			if q.Row != tc.p.Row || q.Col != i {
				t.Errorf("row unit wrong at %v", q)
			}
		}
		for i, q := range units[1] {
			if q.Col != tc.p.Col || q.Row != i {
				t.Errorf("col unit wrong at %v", q)
			}
		}
		for _, q := range units[2] {
			if q.Row < tc.boxRow || q.Row >= tc.boxRow+3 || q.Col < tc.boxCol || q.Col >= tc.boxCol+3 {
				t.Errorf("box unit for %v contains %v", tc.p, q)
			}
		}
	}
}

func TestConflicts(t *testing.T) {
	v := NewStandardValidator()
	b := emptyBoard(t, 9)
	p := model.Position{Row: 4, Col: 4}
	b.SetValue(p, 5)
	b.SetValue(model.Position{Row: 4, Col: 0}, 5) // same row
	b.SetValue(model.Position{Row: 0, Col: 4}, 5) // same column
	b.SetValue(model.Position{Row: 3, Col: 3}, 5) // same box
	b.SetValue(model.Position{Row: 0, Col: 0}, 5) // unrelated
	if got := v.Conflicts(b, p); len(got) != 3 {
		t.Fatalf("got %v, want 3 conflicts", got)
	}
	if got := v.Conflicts(b, model.Position{Row: 8, Col: 8}); got == nil || len(got) != 0 {
		t.Fatalf("empty cell should have an empty, non-nil conflict list, got %#v", got)
	}
}
