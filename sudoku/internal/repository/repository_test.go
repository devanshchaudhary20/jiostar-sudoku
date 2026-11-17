package repository

import (
	"errors"
	"testing"

	"jiostar/sudoku/internal/errs"
	"jiostar/sudoku/internal/model"
	"jiostar/sudoku/internal/randutil"
)

func canonical(t *testing.T, size int) [][]int {
	t.Helper()
	g, err := CanonicalGrid(size)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestSeedsAreValid(t *testing.T) {
	for i, g := range seed9x9 {
		if !model.IsValidSolution(g) {
			t.Errorf("seed9x9[%d] is not a valid solution", i)
		}
	}
	for _, size := range []int{4, 9, 16, 25} {
		if !model.IsValidSolution(canonical(t, size)) {
			t.Errorf("CanonicalGrid(%d) invalid", size)
		}
	}
	if _, err := CanonicalGrid(6); !errors.Is(err, errs.ErrUnsupportedSize) {
		t.Errorf("CanonicalGrid(6) → %v, want ErrUnsupportedSize", err)
	}
}

func TestBaseBoardRepo(t *testing.T) {
	repo := NewInMemoryBaseBoardRepo(randutil.NewLocked(1))
	bad := canonical(t, 4)
	bad[0][0], bad[0][1] = bad[0][1], bad[0][0]
	if err := repo.Add(bad); !errors.Is(err, errs.ErrInvalidBaseBoard) {
		t.Fatalf("want ErrInvalidBaseBoard, got %v", err)
	}
	// A 6x6 Latin square is not a supported board: 6 is not a perfect square.
	six := make([][]int, 6)
	for r := range six {
		six[r] = make([]int, 6)
		for c := range six[r] {
			six[r][c] = (r+c)%6 + 1
		}
	}
	if err := repo.Add(six); !errors.Is(err, errs.ErrUnsupportedSize) {
		t.Fatalf("want ErrUnsupportedSize for 6x6, got %v", err)
	}
	if _, err := repo.GetRandom(4); !errors.Is(err, errs.ErrUnsupportedSize) {
		t.Fatalf("want ErrUnsupportedSize, got %v", err)
	}
	if err := repo.Add(canonical(t, 4)); err != nil {
		t.Fatal(err)
	}
	g, _ := repo.GetRandom(4)
	g[0][0] = 99 // must not corrupt the stored grid
	g2, _ := repo.GetRandom(4)
	if g2[0][0] == 99 {
		t.Fatal("GetRandom must return a copy")
	}
}
