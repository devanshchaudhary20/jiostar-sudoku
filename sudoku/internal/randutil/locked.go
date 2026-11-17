// Package randutil provides a goroutine-safe random source.
//
// *rand.Rand is not safe for concurrent use, and InitGame can be called by
// many users at once, so the generator, shuffler and base-board repository
// share one Locked source. Tests pass a fixed seed for deterministic output.
package randutil

import (
	"math/rand"
	"sync"
)

// Source is the subset of *rand.Rand the service needs.
type Source interface {
	Intn(n int) int
	Perm(n int) []int
}

// Locked wraps *rand.Rand with a mutex.
type Locked struct {
	mu sync.Mutex
	r  *rand.Rand
}

func NewLocked(seed int64) *Locked {
	return &Locked{r: rand.New(rand.NewSource(seed))}
}

func (l *Locked) Intn(n int) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.r.Intn(n)
}

func (l *Locked) Perm(n int) []int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.r.Perm(n)
}
