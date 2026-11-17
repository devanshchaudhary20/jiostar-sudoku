// Package errs holds the sentinel errors returned by the Sudoku service.
// Callers match them with errors.Is; most are wrapped with extra context.
package errs

import "errors"

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrGameNotFound      = errors.New("game not found")
	ErrNotGameOwner      = errors.New("game belongs to another user")
	ErrGameCompleted     = errors.New("game is already completed")
	ErrOutOfBounds       = errors.New("position is out of bounds")
	ErrInvalidValue      = errors.New("value is out of range")
	ErrFixedCell         = errors.New("cell is prefilled and cannot be changed")
	ErrInvalidDifficulty = errors.New("unknown difficulty")
	ErrUnsupportedSize   = errors.New("unsupported board size")
	ErrInvalidBaseBoard  = errors.New("grid is not a valid solved sudoku")
	ErrInvalidName       = errors.New("name must not be empty")
	ErrAlreadyExists     = errors.New("entity already exists")
)
