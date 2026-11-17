package model

import (
	"fmt"
	"sync"
	"time"

	"jiostar/sudoku/internal/errs"
)

type GameStatus string

const (
	InProgress GameStatus = "IN_PROGRESS"
	Completed  GameStatus = "COMPLETED"
)

// MoveResult describes the outcome of one placed move.
type MoveResult struct {
	Valid     bool       `json:"valid"`     // false when the placed value conflicts
	Conflicts []Position `json:"conflicts"` // peers it clashes with
}

// GameState is a consistent snapshot of a game, taken under the game lock.
type GameState struct {
	Status    GameStatus `json:"status"`
	MoveCount int        `json:"moveCount"`
	Board     BoardView  `json:"board"`
}

// Game links one user to one board and orchestrates moves.
//
// Concurrency: mu guards board, status and moveCount. They are unexported so
// they can only be reached through methods that take the lock. ID, UserID,
// Difficulty and CreatedAt never change after creation and need no lock.
// A Game must always be handled as *Game (it contains a mutex).
type Game struct {
	ID         string
	UserID     string
	Difficulty Difficulty
	CreatedAt  time.Time

	mu        sync.RWMutex
	board     *Board
	status    GameStatus
	moveCount int
	validator MoveValidator
}

// NewGame creates a game in progress for the given user and puzzle board.
func NewGame(id, userID string, d Difficulty, board *Board, v MoveValidator, now time.Time) *Game {
	g := &Game{
		ID:         id,
		UserID:     userID,
		Difficulty: d,
		CreatedAt:  now,
		board:      board,
		status:     InProgress,
		validator:  v,
	}
	g.updateStatusLocked()
	return g
}

// ApplyMove places value (1..size) at p, or clears p when value is 0.
//
// Request errors (finished game, out of bounds, value out of range, fixed
// cell) are returned as errors and change nothing. A value that breaks a row,
// column or box constraint is NOT an error: it is placed, flagged Invalid and
// reported with Valid=false.
func (g *Game) ApplyMove(p Position, value int) (MoveResult, GameState, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.status == Completed {
		return MoveResult{}, GameState{}, errs.ErrGameCompleted
	}
	if err := g.validator.ValidateInput(g.board, p, value); err != nil {
		return MoveResult{}, GameState{}, err
	}

	g.board.SetValue(p, value)
	g.reflagLocked(p)
	g.moveCount++
	g.updateStatusLocked()

	conflicts := g.validator.Conflicts(g.board, p)
	result := MoveResult{Valid: len(conflicts) == 0, Conflicts: conflicts}
	return result, g.stateLocked(), nil
}

// State returns a consistent snapshot of the game.
func (g *Game) State() GameState {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.stateLocked()
}

// Status returns the current game status.
func (g *Game) Status() GameStatus {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.status
}

// Verify recomputes every flag and counter from scratch and compares them
// with the incrementally maintained values. It is meant for tests and
// debugging; it returns nil when the game state is consistent.
func (g *Game) Verify() error {
	g.mu.RLock()
	defer g.mu.RUnlock()

	b := g.board
	filled, invalid := 0, 0
	for r := 0; r < b.Size(); r++ {
		for c := 0; c < b.Size(); c++ {
			p := Position{r, c}
			cell := b.Cell(p)
			if cell.Value != 0 {
				filled++
			}
			if cell.Invalid {
				invalid++
			}
			want := !cell.Fixed && cell.Value != 0 && len(g.validator.Conflicts(b, p)) > 0
			if cell.Invalid != want {
				return fmt.Errorf("cell (%d,%d): invalid=%v, want %v", r, c, cell.Invalid, want)
			}
		}
	}
	if filled != b.FilledCount() {
		return fmt.Errorf("filledCount=%d, recount=%d", b.FilledCount(), filled)
	}
	if invalid != b.InvalidCount() {
		return fmt.Errorf("invalidCount=%d, recount=%d", b.InvalidCount(), invalid)
	}
	return nil
}

// reflagLocked re-evaluates the conflict flag of every non-fixed cell that
// shares a unit with p. Changing p can only create or resolve conflicts among
// those cells, and Conflicts(q) checks all of q's units, so a cell that still
// clashes elsewhere stays flagged. Fixed cells come from a valid solution and
// are never flagged: when an entry clashes with a given, only the entry is.
func (g *Game) reflagLocked(p Position) {
	seen := make(map[Position]bool)
	for _, unit := range g.validator.Units(g.board, p) {
		for _, q := range unit {
			if seen[q] {
				continue
			}
			seen[q] = true
			cell := g.board.Cell(q)
			if cell.Fixed {
				continue
			}
			invalid := cell.Value != 0 && len(g.validator.Conflicts(g.board, q)) > 0
			g.board.SetInvalid(q, invalid)
		}
	}
}

// updateStatusLocked marks the game completed once the board is full and
// conflict-free. A full board with conflicts stays in progress.
func (g *Game) updateStatusLocked() {
	if g.board.IsFull() && !g.board.HasConflicts() {
		g.status = Completed
	}
}

func (g *Game) stateLocked() GameState {
	return GameState{Status: g.status, MoveCount: g.moveCount, Board: g.board.Snapshot()}
}
