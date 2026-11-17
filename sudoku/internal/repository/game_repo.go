package repository

import (
	"fmt"
	"sync"

	"jiostar/sudoku/internal/errs"
	"jiostar/sudoku/internal/model"
)

type GameRepository interface {
	Save(g *model.Game) error
	Get(id string) (*model.Game, error)
	ListByUser(userID string) ([]*model.Game, error)
}

// InMemoryGameRepo indexes games by ID and by user. Both maps share one
// RWMutex. The lock covers only the map access: Get returns the *Game and
// releases the lock immediately; the game's own lock guards what happens next.
type InMemoryGameRepo struct {
	mu     sync.RWMutex
	byID   map[string]*model.Game
	byUser map[string][]string
}

func NewInMemoryGameRepo() *InMemoryGameRepo {
	return &InMemoryGameRepo{
		byID:   make(map[string]*model.Game),
		byUser: make(map[string][]string),
	}
}

func (r *InMemoryGameRepo) Save(g *model.Game) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byID[g.ID]; exists {
		return fmt.Errorf("%w: game %s", errs.ErrAlreadyExists, g.ID)
	}
	r.byID[g.ID] = g
	r.byUser[g.UserID] = append(r.byUser[g.UserID], g.ID)
	return nil
}

func (r *InMemoryGameRepo) Get(id string) (*model.Game, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	g, ok := r.byID[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", errs.ErrGameNotFound, id)
	}
	return g, nil
}

func (r *InMemoryGameRepo) ListByUser(userID string) ([]*model.Game, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := r.byUser[userID]
	games := make([]*model.Game, 0, len(ids))
	for _, id := range ids {
		games = append(games, r.byID[id])
	}
	return games, nil
}
