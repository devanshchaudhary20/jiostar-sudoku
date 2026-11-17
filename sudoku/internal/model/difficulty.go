package model

import (
	"fmt"
	"strings"

	"jiostar/sudoku/internal/errs"
)

// Difficulty is the puzzle difficulty. There are always exactly three levels,
// each defined as a fraction of the board's total cell count so that it scales
// to any board size.
type Difficulty string

const (
	Easy   Difficulty = "EASY"
	Medium Difficulty = "MEDIUM"
	Hard   Difficulty = "HARD"
)

const (
	// hardMinPercent is the floor for Hard (22 clues on 9x9). Puzzles with
	// very few clues are almost never uniquely solvable, so we stop well above 17.
	hardMinPercent = 27
	// easyMaxPercent is the ceiling for Easy (64 clues on 9x9), so an easy
	// puzzle still leaves something to solve.
	easyMaxPercent = 80
)

// ParseDifficulty converts user input such as "medium" into a Difficulty.
func ParseDifficulty(s string) (Difficulty, error) {
	d := Difficulty(strings.ToUpper(strings.TrimSpace(s)))
	switch d {
	case Easy, Medium, Hard:
		return d, nil
	}
	return "", fmt.Errorf("%w: %q (want EASY, MEDIUM or HARD)", errs.ErrInvalidDifficulty, s)
}

// ClueRange returns the inclusive [min, max] number of prefilled cells for a
// board with totalCells = size*size.
//
//	Hard:   [ceil(0.27N), ceil(N/3)-1]
//	Medium: [ceil(N/3),   floor(2N/3)]   (both thirds inclusive)
//	Easy:   [floor(2N/3)+1, floor(0.8N)]
//
// On 9x9 (N=81) this gives Hard 22-26, Medium 27-54, Easy 55-64.
// Integer arithmetic is used throughout to avoid floating-point edge cases.
func (d Difficulty) ClueRange(totalCells int) (min, max int, err error) {
	if totalCells <= 0 {
		return 0, 0, fmt.Errorf("%w: %d cells", errs.ErrUnsupportedSize, totalCells)
	}
	firstMedium := ceilDiv(totalCells, 3)
	lastMedium := 2 * totalCells / 3

	switch d {
	case Hard:
		min, max = ceilDiv(hardMinPercent*totalCells, 100), firstMedium-1
	case Medium:
		min, max = firstMedium, lastMedium
	case Easy:
		min, max = lastMedium+1, easyMaxPercent*totalCells/100
	default:
		return 0, 0, fmt.Errorf("%w: %q", errs.ErrInvalidDifficulty, string(d))
	}
	if min > max {
		return 0, 0, fmt.Errorf("%w: no %s range for %d cells", errs.ErrUnsupportedSize, d, totalCells)
	}
	return min, max, nil
}

func ceilDiv(a, b int) int { return (a + b - 1) / b }
