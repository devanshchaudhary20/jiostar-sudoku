package model

import (
	"fmt"

	"jiostar/sudoku/internal/errs"
)

// Board holds the grid and its counters. It is pure data: it knows nothing
// about sudoku rules (those live in a MoveValidator) and it is not
// goroutine-safe on its own (the owning Game guards it).
//
// The board size is the only input; the box size is derived from it
// (√size), so a 9x9 board has nine 3x3 boxes and a 16x16 board sixteen 4x4 boxes.
type Board struct {
	size         int // side of the board: 4, 9, 16, ...
	boxSize      int // side of one box: √size
	cells        [][]Cell
	filledCount  int // cells with a non-zero value
	invalidCount int // cells with Invalid == true
}

// NewBoard returns an empty size×size board. size must be a perfect square ≥ 4.
func NewBoard(size int) (*Board, error) {
	boxSize, err := BoxSizeFor(size)
	if err != nil {
		return nil, err
	}
	cells := make([][]Cell, size)
	for r := range cells {
		cells[r] = make([]Cell, size)
	}
	return &Board{size: size, boxSize: boxSize, cells: cells}, nil
}

// NewBoardFromGrid builds a board from a square puzzle grid; the board size
// is len(grid). Every non-zero value becomes a fixed (prefilled) cell; zeros
// are empty, editable cells.
func NewBoardFromGrid(grid [][]int) (*Board, error) {
	b, err := NewBoard(len(grid))
	if err != nil {
		return nil, err
	}
	for r, row := range grid {
		if len(row) != b.size {
			return nil, fmt.Errorf("%w: row %d has %d columns, want %d", errs.ErrUnsupportedSize, r, len(row), b.size)
		}
		for c, v := range row {
			if v < 0 || v > b.size {
				return nil, fmt.Errorf("%w: %d at (%d,%d)", errs.ErrInvalidValue, v, r, c)
			}
			if v != 0 {
				b.cells[r][c] = Cell{Value: v, Fixed: true}
				b.filledCount++
			}
		}
	}
	return b, nil
}

// Size is the side of the board (9 for 9x9).
func (b *Board) Size() int { return b.size }

// BoxSize is the side of one box, derived as √Size (3 for 9x9).
func (b *Board) BoxSize() int { return b.boxSize }

func (b *Board) FilledCount() int  { return b.filledCount }
func (b *Board) InvalidCount() int { return b.invalidCount }

// InBounds reports whether p lies on the board.
func (b *Board) InBounds(p Position) bool {
	return p.Row >= 0 && p.Row < b.size && p.Col >= 0 && p.Col < b.size
}

// Cell returns a copy of the cell at p. p must be in bounds.
func (b *Board) Cell(p Position) Cell { return b.cells[p.Row][p.Col] }

// SetValue writes v (0 clears) at p and keeps filledCount in step.
// The caller is responsible for validating p, v and that the cell isn't fixed.
func (b *Board) SetValue(p Position, v int) {
	cell := &b.cells[p.Row][p.Col]
	switch {
	case cell.Value == 0 && v != 0:
		b.filledCount++
	case cell.Value != 0 && v == 0:
		b.filledCount--
	}
	cell.Value = v
}

// SetInvalid sets the conflict flag at p and keeps invalidCount in step.
func (b *Board) SetInvalid(p Position, invalid bool) {
	cell := &b.cells[p.Row][p.Col]
	if cell.Invalid == invalid {
		return
	}
	if invalid {
		b.invalidCount++
	} else {
		b.invalidCount--
	}
	cell.Invalid = invalid
}

// IsFull reports whether every cell holds a value. O(1).
func (b *Board) IsFull() bool { return b.filledCount == b.size*b.size }

// HasConflicts reports whether any cell is flagged invalid. O(1).
func (b *Board) HasConflicts() bool { return b.invalidCount > 0 }

// Snapshot returns a deep copy suitable for returning to callers.
func (b *Board) Snapshot() BoardView {
	v := BoardView{
		Size:    b.size,
		BoxSize: b.boxSize,
		Grid:    make([][]int, b.size),
		Fixed:   make([][]bool, b.size),
		Invalid: make([][]bool, b.size),
	}
	for r, row := range b.cells {
		v.Grid[r] = make([]int, b.size)
		v.Fixed[r] = make([]bool, b.size)
		v.Invalid[r] = make([]bool, b.size)
		for c, cell := range row {
			v.Grid[r][c] = cell.Value
			v.Fixed[r][c] = cell.Fixed
			v.Invalid[r][c] = cell.Invalid
		}
	}
	return v
}
