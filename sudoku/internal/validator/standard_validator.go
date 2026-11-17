// Package validator contains MoveValidator implementations.
package validator

import (
	"fmt"

	"jiostar/sudoku/internal/errs"
	"jiostar/sudoku/internal/model"
)

// StandardValidator implements classic sudoku rules: each value appears at
// most once per row, column and box.
//
// To add a variant (e.g. diagonal sudoku), write a new type whose Units also
// returns the diagonals containing p, and implement Conflicts as
// model.ConflictingPeers(b, p, v.Units(b, p)). Game, Board and the controller
// need no changes.
type StandardValidator struct{}

func NewStandardValidator() *StandardValidator { return &StandardValidator{} }

var _ model.MoveValidator = (*StandardValidator)(nil)

func (v *StandardValidator) ValidateInput(b *model.Board, p model.Position, value int) error {
	if !b.InBounds(p) {
		return fmt.Errorf("%w: (%d,%d) on a %dx%d board", errs.ErrOutOfBounds, p.Row, p.Col, b.Size(), b.Size())
	}
	if value < 0 || value > b.Size() {
		return fmt.Errorf("%w: %d (allowed 1-%d, or 0 to clear)", errs.ErrInvalidValue, value, b.Size())
	}
	if b.Cell(p).Fixed {
		return fmt.Errorf("%w: (%d,%d)", errs.ErrFixedCell, p.Row, p.Col)
	}
	return nil
}

func (v *StandardValidator) Units(b *model.Board, p model.Position) [][]model.Position {
	size, bs := b.Size(), b.BoxSize()
	row := make([]model.Position, 0, size)
	col := make([]model.Position, 0, size)
	box := make([]model.Position, 0, size)

	r0, c0 := (p.Row/bs)*bs, (p.Col/bs)*bs
	for i := 0; i < size; i++ {
		row = append(row, model.Position{Row: p.Row, Col: i})
		col = append(col, model.Position{Row: i, Col: p.Col})
		box = append(box, model.Position{Row: r0 + i/bs, Col: c0 + i%bs})
	}
	return [][]model.Position{row, col, box}
}

func (v *StandardValidator) Conflicts(b *model.Board, p model.Position) []model.Position {
	return model.ConflictingPeers(b, p, v.Units(b, p))
}
