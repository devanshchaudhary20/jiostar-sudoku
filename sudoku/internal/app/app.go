// Package app wires the default in-memory implementation together.
package app

import (
	"jiostar/sudoku/internal/controller"
	"jiostar/sudoku/internal/generator"
	"jiostar/sudoku/internal/randutil"
	"jiostar/sudoku/internal/repository"
	"jiostar/sudoku/internal/validator"
)

// New builds a SudokuController with in-memory repositories, seeded base
// boards, the rotation shuffler and the standard validator. Pass a fixed
// seed for deterministic puzzles (tests), or time.Now().UnixNano().
func New(seed int64) (*controller.SudokuController, error) {
	rng := randutil.NewLocked(seed)

	baseRepo := repository.NewInMemoryBaseBoardRepo(rng)
	if err := repository.SeedDefaults(baseRepo); err != nil {
		return nil, err
	}

	gen := generator.NewRandomRemovalGenerator(baseRepo, generator.NewRotationShuffler(rng), rng)

	return controller.NewSudokuController(
		repository.NewInMemoryUserRepo(),
		repository.NewInMemoryGameRepo(),
		gen,
		validator.NewStandardValidator(),
		&controller.SequentialIDGenerator{},
	), nil
}
