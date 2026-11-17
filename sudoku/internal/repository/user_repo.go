package repository

import (
	"fmt"
	"sync"

	"jiostar/sudoku/internal/errs"
	"jiostar/sudoku/internal/model"
)

type UserRepository interface {
	Save(u *model.User) error
	Get(id string) (*model.User, error)
}

type InMemoryUserRepo struct {
	mu   sync.RWMutex
	byID map[string]*model.User
}

func NewInMemoryUserRepo() *InMemoryUserRepo {
	return &InMemoryUserRepo{byID: make(map[string]*model.User)}
}

func (r *InMemoryUserRepo) Save(u *model.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byID[u.ID]; exists {
		return fmt.Errorf("%w: user %s", errs.ErrAlreadyExists, u.ID)
	}
	r.byID[u.ID] = u
	return nil
}

func (r *InMemoryUserRepo) Get(id string) (*model.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.byID[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", errs.ErrUserNotFound, id)
	}
	return u, nil
}
