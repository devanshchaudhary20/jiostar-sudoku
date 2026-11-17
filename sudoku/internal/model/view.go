package model

// BoardView is an immutable, JSON-friendly copy of the board returned to callers.
type BoardView struct {
	Size    int      `json:"size"`    // board side, e.g. 9
	BoxSize int      `json:"boxSize"` // derived √size, e.g. 3; lets clients draw box borders
	Grid    [][]int  `json:"grid"`    // 0 = empty
	Fixed   [][]bool `json:"fixed"`   // true = prefilled, locked
	Invalid [][]bool `json:"invalid"` // true = conflicts with a peer
}
