package model

import (
	"errors"
	"testing"

	"jiostar/sudoku/internal/errs"
)

// Box size is derived from the board size: a size×size board has size boxes of √size×√size.
func TestBoxSizeDerivedFromBoardSize(t *testing.T) {
	for size, want := range map[int]int{4: 2, 9: 3, 16: 4, 25: 5, 36: 6} {
		b, err := NewBoard(size)
		if err != nil {
			t.Fatalf("NewBoard(%d): %v", size, err)
		}
		if b.Size() != size || b.BoxSize() != want {
			t.Errorf("NewBoard(%d): size=%d boxSize=%d, want %d", size, b.Size(), b.BoxSize(), want)
		}
		// size boxes in total, each with size cells
		if boxes := (size / want) * (size / want); boxes != size {
			t.Errorf("%dx%d board has %d boxes, want %d", size, size, boxes, size)
		}
	}
	for _, size := range []int{-4, 0, 1, 2, 3, 6, 8, 10, 12, 15} {
		if _, err := NewBoard(size); !errors.Is(err, errs.ErrUnsupportedSize) {
			t.Errorf("NewBoard(%d) → %v, want ErrUnsupportedSize", size, err)
		}
	}
}

func TestBoardCounters(t *testing.T) {
	b, err := NewBoard(4)
	if err != nil {
		t.Fatal(err)
	}
	p := Position{1, 2}
	b.SetValue(p, 3)
	b.SetValue(p, 4) // overwrite: still one filled cell
	if b.FilledCount() != 1 {
		t.Fatalf("filled=%d, want 1", b.FilledCount())
	}
	b.SetInvalid(p, true)
	b.SetInvalid(p, true) // idempotent
	if b.InvalidCount() != 1 || !b.HasConflicts() {
		t.Fatalf("invalid=%d, want 1", b.InvalidCount())
	}
	b.SetInvalid(p, false)
	b.SetValue(p, 0)
	if b.FilledCount() != 0 || b.InvalidCount() != 0 {
		t.Fatalf("filled=%d invalid=%d, want 0 0", b.FilledCount(), b.InvalidCount())
	}
}

func TestNewBoardFromGrid(t *testing.T) {
	grid := [][]int{{1, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 4}}
	b, err := NewBoardFromGrid(grid)
	if err != nil {
		t.Fatal(err)
	}
	if b.BoxSize() != 2 || !b.Cell(Position{0, 0}).Fixed || b.Cell(Position{0, 1}).Fixed || b.FilledCount() != 2 {
		t.Fatal("box size, fixed flags or filled count wrong")
	}
	grid[0][1] = 5
	if _, err := NewBoardFromGrid(grid); !errors.Is(err, errs.ErrInvalidValue) {
		t.Fatalf("want ErrInvalidValue, got %v", err)
	}
	ragged := [][]int{{1, 0, 0, 0}, {0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}}
	if _, err := NewBoardFromGrid(ragged); !errors.Is(err, errs.ErrUnsupportedSize) {
		t.Fatalf("want ErrUnsupportedSize for ragged grid, got %v", err)
	}
}

func TestSnapshotIsDeepCopy(t *testing.T) {
	b, _ := NewBoard(4)
	v := b.Snapshot()
	v.Grid[0][0] = 9
	if b.Cell(Position{0, 0}).Value != 0 {
		t.Fatal("snapshot shares memory with the board")
	}
	if v.Size != 4 || v.BoxSize != 2 {
		t.Fatalf("view size=%d boxSize=%d", v.Size, v.BoxSize)
	}
}

func TestIsValidSolution(t *testing.T) {
	good := [][]int{{1, 2, 3, 4}, {3, 4, 1, 2}, {2, 1, 4, 3}, {4, 3, 2, 1}}
	if !IsValidSolution(good) {
		t.Fatal("valid grid rejected")
	}
	// Valid rows and columns, but the top-left 2x2 box has 1,2,2,1.
	badBox := [][]int{{1, 2, 3, 4}, {2, 1, 4, 3}, {3, 4, 1, 2}, {4, 3, 2, 1}}
	if IsValidSolution(badBox) {
		t.Fatal("box violation accepted")
	}
}
