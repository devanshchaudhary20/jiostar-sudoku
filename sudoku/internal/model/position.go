package model

// Position is a zero-based (row, column) coordinate on the board.
type Position struct {
	Row int `json:"row"`
	Col int `json:"col"`
}
