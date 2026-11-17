package model

import (
	"fmt"

	"jiostar/sudoku/internal/errs"
)

// MinBoardSize is the smallest supported board (4x4 with 2x2 boxes).
const MinBoardSize = 4

// BoxSizeFor derives the box side from the board side: a size×size board
// has size boxes of √size × √size each (4x4 → 2x2 boxes, 9x9 → 3x3,
// 16x16 → 4x4). size must be a perfect square of at least 4.
func BoxSizeFor(size int) (int, error) {
	for bs := 2; bs*bs <= size; bs++ {
		if bs*bs == size {
			return bs, nil
		}
	}
	return 0, fmt.Errorf("%w: %dx%d (board size must be a perfect square ≥ %d, e.g. 4, 9, 16)",
		errs.ErrUnsupportedSize, size, size, MinBoardSize)
}

// IsValidSolution reports whether grid is a completely and correctly solved
// square sudoku: every row, column and box contains each of 1..size exactly
// once. The box size is derived from the grid's size.
func IsValidSolution(grid [][]int) bool {
	size := len(grid)
	boxSize, err := BoxSizeFor(size)
	if err != nil {
		return false
	}
	for _, row := range grid {
		if len(row) != size {
			return false
		}
	}

	// unitOK checks one unit given a function that yields its i-th value.
	unitOK := func(at func(i int) int) bool {
		seen := make([]bool, size+1)
		for i := 0; i < size; i++ {
			v := at(i)
			if v < 1 || v > size || seen[v] {
				return false
			}
			seen[v] = true
		}
		return true
	}

	for k := 0; k < size; k++ {
		k := k
		row := func(i int) int { return grid[k][i] }
		col := func(i int) int { return grid[i][k] }
		r0, c0 := (k/boxSize)*boxSize, (k%boxSize)*boxSize
		box := func(i int) int { return grid[r0+i/boxSize][c0+i%boxSize] }
		if !unitOK(row) || !unitOK(col) || !unitOK(box) {
			return false
		}
	}
	return true
}

// CopyGrid returns a deep copy of grid.
func CopyGrid(grid [][]int) [][]int {
	out := make([][]int, len(grid))
	for r, row := range grid {
		out[r] = append([]int(nil), row...)
	}
	return out
}
