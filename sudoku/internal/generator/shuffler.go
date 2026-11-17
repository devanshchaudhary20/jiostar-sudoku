// Package generator turns stored solved grids into playable puzzles.
package generator

import (
	"jiostar/sudoku/internal/model"
	"jiostar/sudoku/internal/randutil"
)

// Shuffler transforms a solved grid in place into another solved grid.
// The board size is len(grid); the box size is derived from it.
type Shuffler interface {
	Shuffle(grid [][]int)
}

// RotationShuffler applies a fixed, ordered pipeline of transforms, each of
// which provably maps a valid solved grid to another valid solved grid:
//
//  1. Relabel digits with a random permutation of 1..size.
//  2. Rotate the rows inside every band.
//  3. Rotate the columns inside every stack.
//  4. Rotate the bands.
//  5. Rotate the stacks.
//
// A band is a horizontal strip of √size rows (one row of boxes); a stack is
// a vertical strip of √size columns. Rows never leave their band and columns
// never leave their stack, which is what keeps every box valid. Moving a row
// into another band, or rotating the contents of a single box, would break
// the grid and is deliberately not offered.
type RotationShuffler struct {
	rng randutil.Source
}

func NewRotationShuffler(rng randutil.Source) *RotationShuffler {
	return &RotationShuffler{rng: rng}
}

func (s *RotationShuffler) Shuffle(grid [][]int) {
	boxSize := boxSizeOf(grid)
	RelabelDigits(grid, s.rng.Perm(len(grid)))
	for band := 0; band < boxSize; band++ {
		RotateRowsInBand(grid, band, s.rng.Intn(boxSize))
	}
	for stack := 0; stack < boxSize; stack++ {
		RotateColsInStack(grid, stack, s.rng.Intn(boxSize))
	}
	RotateBands(grid, s.rng.Intn(boxSize))
	RotateStacks(grid, s.rng.Intn(boxSize))
}

// The transforms below are exported and take explicit offsets so each one
// can be tested on its own. An offset of 0 leaves the grid unchanged. Each
// derives the box size from the grid; grids come from the base-board
// repository, which only accepts perfect-square sizes.

// boxSizeOf returns √len(grid). It panics on a non-square size, which would
// be a programming error since every grid is validated on the way in.
func boxSizeOf(grid [][]int) int {
	bs, err := model.BoxSizeFor(len(grid))
	if err != nil {
		panic(err)
	}
	return bs
}

// RelabelDigits replaces every value v with perm[v-1]+1. perm must be a
// permutation of 0..size-1, so the mapping is a bijection and distinct
// values stay distinct in every row, column and box.
func RelabelDigits(grid [][]int, perm []int) {
	for _, row := range grid {
		for c, v := range row {
			if v > 0 {
				row[c] = perm[v-1] + 1
			}
		}
	}
}

// RotateRowsInBand cyclically shifts the rows of one band by k: new row i
// of the band is old row (i+k) mod boxSize. Row contents are unchanged,
// columns are only reordered, and each box keeps the same set of values.
func RotateRowsInBand(grid [][]int, band, k int) {
	boxSize := boxSizeOf(grid)
	start := band * boxSize
	rows := make([][]int, boxSize)
	for i := 0; i < boxSize; i++ {
		rows[i] = grid[start+(i+k)%boxSize]
	}
	copy(grid[start:start+boxSize], rows)
}

// RotateColsInStack is RotateRowsInBand with rows and columns swapped.
func RotateColsInStack(grid [][]int, stack, k int) {
	boxSize := boxSizeOf(grid)
	start := stack * boxSize
	tmp := make([]int, boxSize)
	for _, row := range grid {
		for i := 0; i < boxSize; i++ {
			tmp[i] = row[start+(i+k)%boxSize]
		}
		copy(row[start:start+boxSize], tmp)
	}
}

// RotateBands cyclically shifts whole bands by k. Boxes move intact, rows
// are unchanged and columns are only reordered.
func RotateBands(grid [][]int, k int) {
	boxSize := boxSizeOf(grid)
	rows := make([][]int, len(grid))
	for b := 0; b < boxSize; b++ {
		src := (b + k) % boxSize
		for i := 0; i < boxSize; i++ {
			rows[b*boxSize+i] = grid[src*boxSize+i]
		}
	}
	copy(grid, rows)
}

// RotateStacks is RotateBands with rows and columns swapped.
func RotateStacks(grid [][]int, k int) {
	boxSize := boxSizeOf(grid)
	tmp := make([]int, len(grid))
	for _, row := range grid {
		for s := 0; s < boxSize; s++ {
			src := (s + k) % boxSize
			for i := 0; i < boxSize; i++ {
				tmp[s*boxSize+i] = row[src*boxSize+i]
			}
		}
		copy(row, tmp)
	}
}
