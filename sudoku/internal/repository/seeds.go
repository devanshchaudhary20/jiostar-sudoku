package repository

import "jiostar/sudoku/internal/model"

// Seed grids. Every one is validated by Add, so a typo fails fast at startup.
var seed9x9 = [][][]int{
	{
		{5, 3, 4, 6, 7, 8, 9, 1, 2},
		{6, 7, 2, 1, 9, 5, 3, 4, 8},
		{1, 9, 8, 3, 4, 2, 5, 6, 7},
		{8, 5, 9, 7, 6, 1, 4, 2, 3},
		{4, 2, 6, 8, 5, 3, 7, 9, 1},
		{7, 1, 3, 9, 2, 4, 8, 5, 6},
		{9, 6, 1, 5, 3, 7, 2, 8, 4},
		{2, 8, 7, 4, 1, 9, 6, 3, 5},
		{3, 4, 5, 2, 8, 6, 1, 7, 9},
	},
	{
		{1, 2, 3, 4, 5, 6, 7, 8, 9},
		{4, 5, 6, 7, 8, 9, 1, 2, 3},
		{7, 8, 9, 1, 2, 3, 4, 5, 6},
		{2, 1, 4, 3, 6, 5, 8, 9, 7},
		{3, 6, 5, 8, 9, 7, 2, 1, 4},
		{8, 9, 7, 2, 1, 4, 3, 6, 5},
		{5, 3, 1, 6, 4, 2, 9, 7, 8},
		{6, 4, 2, 9, 7, 8, 5, 3, 1},
		{9, 7, 8, 5, 3, 1, 6, 4, 2},
	},
}

// CanonicalGrid returns the standard pattern solution for a size×size board
// (size a perfect square), with boxSize = √size:
// value(r, c) = (boxSize*(r mod boxSize) + r/boxSize + c) mod size + 1.
// It is used to seed sizes without hand-written grids (4x4, 16x16).
func CanonicalGrid(size int) ([][]int, error) {
	boxSize, err := model.BoxSizeFor(size)
	if err != nil {
		return nil, err
	}
	grid := make([][]int, size)
	for r := range grid {
		grid[r] = make([]int, size)
		for c := range grid[r] {
			grid[r][c] = (boxSize*(r%boxSize)+r/boxSize+c)%size + 1
		}
	}
	return grid, nil
}

// DefaultBoardSizes are the board sizes seeded at startup.
var DefaultBoardSizes = []int{4, 9, 16}

// SeedDefaults loads the built-in grids: two hand-written 9x9 grids plus
// canonical 4x4, 9x9 and 16x16 grids. The shuffler gives each one billions
// of variants, so a handful of seeds is plenty.
func SeedDefaults(repo BaseBoardRepository) error {
	for _, g := range seed9x9 {
		if err := repo.Add(g); err != nil {
			return err
		}
	}
	for _, size := range DefaultBoardSizes {
		grid, err := CanonicalGrid(size)
		if err != nil {
			return err
		}
		if err := repo.Add(grid); err != nil {
			return err
		}
	}
	return nil
}
