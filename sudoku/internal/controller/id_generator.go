package controller

import (
	"fmt"
	"sync/atomic"
)

// IDGenerator produces unique IDs such as "user-1" or "game-7".
type IDGenerator interface {
	Next(prefix string) string
}

// SequentialIDGenerator uses one atomic counter shared by all prefixes.
type SequentialIDGenerator struct {
	n atomic.Int64
}

func (g *SequentialIDGenerator) Next(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, g.n.Add(1))
}
