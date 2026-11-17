package model

import (
	"errors"
	"testing"

	"jiostar/sudoku/internal/errs"
)

func TestClueRange(t *testing.T) {
	tests := []struct {
		cells    int
		d        Difficulty
		min, max int
	}{
		{16, Hard, 5, 5}, {16, Medium, 6, 10}, {16, Easy, 11, 12},
		{81, Hard, 22, 26}, {81, Medium, 27, 54}, {81, Easy, 55, 64},
		{256, Hard, 70, 85}, {256, Medium, 86, 170}, {256, Easy, 171, 204},
	}
	for _, tt := range tests {
		min, max, err := tt.d.ClueRange(tt.cells)
		if err != nil || min != tt.min || max != tt.max {
			t.Errorf("%s(%d) = [%d,%d] %v, want [%d,%d]", tt.d, tt.cells, min, max, err, tt.min, tt.max)
		}
	}
}

// The user's rule on 9x9: <27 hard, >54 easy, 27..54 inclusive medium.
func TestBoundariesOn9x9(t *testing.T) {
	bandOf := func(clues int) Difficulty {
		for _, d := range []Difficulty{Hard, Medium, Easy} {
			min, max, _ := d.ClueRange(81)
			if clues >= min && clues <= max {
				return d
			}
		}
		return ""
	}
	for clues, want := range map[int]Difficulty{26: Hard, 27: Medium, 54: Medium, 55: Easy} {
		if got := bandOf(clues); got != want {
			t.Errorf("%d clues → %q, want %s", clues, got, want)
		}
	}
}

func TestParseDifficulty(t *testing.T) {
	if d, err := ParseDifficulty(" medium "); err != nil || d != Medium {
		t.Errorf("got %q, %v", d, err)
	}
	if _, err := ParseDifficulty("insane"); !errors.Is(err, errs.ErrInvalidDifficulty) {
		t.Errorf("want ErrInvalidDifficulty, got %v", err)
	}
}
