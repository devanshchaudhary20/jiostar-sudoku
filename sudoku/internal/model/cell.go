package model

// Cell is the state of one position on the board.
// Future per-cell state (pencil marks, last-modified time, hints) belongs here.
type Cell struct {
	Value   int  // 0 = empty, 1..size otherwise
	Fixed   bool // prefilled; cannot be changed
	Invalid bool // conflicts with another cell in one of its constraint groups
}
