// Package repository holds in-memory stores. Each one guards its maps with
// a sync.RWMutex held only for the map operation itself (see DESIGN.md §9.1).
package repository

import (
	"fmt"
	"sync"

	"jiostar/sudoku/internal/errs"
	"jiostar/sudoku/internal/model"
	"jiostar/sudoku/internal/randutil"
)

// BaseBoardRepository stores fully solved grids, keyed by side length.
type BaseBoardRepository interface {
	// Add stores a copy of grid. It rejects grids that aren't complete,
	// valid solutions of a perfect-square size.
	Add(grid [][]int) error
	// GetRandom returns a deep copy of a random stored grid of that size.
	GetRandom(size int) ([][]int, error)
}

type InMemoryBaseBoardRepo struct {
	mu     sync.RWMutex
	bySize map[int][][][]int
	rng    randutil.Source
}

func NewInMemoryBaseBoardRepo(rng randutil.Source) *InMemoryBaseBoardRepo {
	return &InMemoryBaseBoardRepo{bySize: make(map[int][][][]int), rng: rng}
}

func (r *InMemoryBaseBoardRepo) Add(grid [][]int) error {
	size := len(grid)
	if _, err := model.BoxSizeFor(size); err != nil {
		return err
	}
	if !model.IsValidSolution(grid) {
		return fmt.Errorf("%w: %dx%d seed grid", errs.ErrInvalidBaseBoard, size, size)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bySize[size] = append(r.bySize[size], model.CopyGrid(grid))
	return nil
}

func (r *InMemoryBaseBoardRepo) GetRandom(size int) ([][]int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	grids := r.bySize[size]
	if len(grids) == 0 {
		return nil, fmt.Errorf("%w: no base board for %dx%d", errs.ErrUnsupportedSize, size, size)
	}
	return model.CopyGrid(grids[r.rng.Intn(len(grids))]), nil
}
