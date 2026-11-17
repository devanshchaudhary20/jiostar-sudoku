package generator

import (
	"fmt"

	"jiostar/sudoku/internal/errs"
	"jiostar/sudoku/internal/model"
	"jiostar/sudoku/internal/randutil"
	"jiostar/sudoku/internal/repository"
)

// PuzzleGenerator creates a new size×size puzzle for a difficulty.
// size must be a perfect square (4, 9, 16, ...); the box size is √size.
type PuzzleGenerator interface {
	Generate(size int, d model.Difficulty) (*model.Board, error)
}

// RandomRemovalGenerator picks a stored solved grid, shuffles it with a
// validity-preserving Shuffler, and blanks random cells until the number of
// clues falls in the difficulty band.
//
// Removing cells from a valid solution can never make the puzzle
// unsolvable (the original grid still fits). Uniqueness of the solution is
// not guaranteed; a UniqueRemovalGenerator can be added behind the same
// interface if that is ever needed.
type RandomRemovalGenerator struct {
	baseRepo repository.BaseBoardRepository
	shuffler Shuffler
	rng      randutil.Source
}

func NewRandomRemovalGenerator(baseRepo repository.BaseBoardRepository, shuffler Shuffler, rng randutil.Source) *RandomRemovalGenerator {
	return &RandomRemovalGenerator{baseRepo: baseRepo, shuffler: shuffler, rng: rng}
}

func (g *RandomRemovalGenerator) Generate(size int, d model.Difficulty) (*model.Board, error) {
	if _, err := model.BoxSizeFor(size); err != nil {
		return nil, err
	}
	total := size * size

	minClues, maxClues, err := d.ClueRange(total)
	if err != nil {
		return nil, err
	}

	grid, err := g.baseRepo.GetRandom(size) // already a deep copy
	if err != nil {
		return nil, err
	}

	g.shuffler.Shuffle(grid)
	// Safety net: turns any transform bug into a visible error instead of a broken game.
	if !model.IsValidSolution(grid) {
		return nil, fmt.Errorf("%w: shuffled grid failed validation", errs.ErrInvalidBaseBoard)
	}

	clues := minClues + g.rng.Intn(maxClues-minClues+1)
	order := g.rng.Perm(total)
	for _, idx := range order[:total-clues] {
		grid[idx/size][idx%size] = 0
	}
	return model.NewBoardFromGrid(grid)
}
