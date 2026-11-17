package model

// MoveValidator holds the rules of a sudoku variant. It lives in package model
// (implementations live elsewhere) so that Game can depend on the interface
// without an import cycle.
type MoveValidator interface {
	// ValidateInput rejects requests that must never be placed on the board:
	// out-of-bounds positions, values outside 1..size (0 = clear is allowed)
	// and writes to fixed cells.
	ValidateInput(b *Board, p Position, value int) error

	// Units returns every constraint group (row, column, box, ...) containing p.
	// Each unit includes p itself.
	Units(b *Board, p Position) [][]Position

	// Conflicts returns the peers of p that hold the same non-zero value.
	// It returns an empty (non-nil) slice when there are none.
	Conflicts(b *Board, p Position) []Position
}

// ConflictingPeers is a helper for MoveValidator implementations: given the
// units that contain p, it returns the distinct cells other than p that hold
// p's non-zero value. A variant validator only has to supply its own units.
func ConflictingPeers(b *Board, p Position, units [][]Position) []Position {
	out := []Position{}
	value := b.Cell(p).Value
	if value == 0 {
		return out
	}
	seen := map[Position]bool{p: true}
	for _, unit := range units {
		for _, q := range unit {
			if seen[q] {
				continue
			}
			seen[q] = true
			if b.Cell(q).Value == value {
				out = append(out, q)
			}
		}
	}
	return out
}
