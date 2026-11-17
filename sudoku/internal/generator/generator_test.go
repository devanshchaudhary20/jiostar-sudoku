package generator

import (
	"testing"

	"jiostar/sudoku/internal/model"
	"jiostar/sudoku/internal/randutil"
	"jiostar/sudoku/internal/repository"
)

var boardSizes = []int{4, 9, 16}

func canonical(t *testing.T, size int) [][]int {
	t.Helper()
	g, err := repository.CanonicalGrid(size)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestEachTransformPreservesValidity(t *testing.T) {
	rng := randutil.NewLocked(1)
	for _, size := range boardSizes {
		bs, _ := model.BoxSizeFor(size)
		transforms := map[string]func(g [][]int){
			"relabel":     func(g [][]int) { RelabelDigits(g, rng.Perm(size)) },
			"rowsInBand":  func(g [][]int) { RotateRowsInBand(g, rng.Intn(bs), 1+rng.Intn(bs-1)) },
			"colsInStack": func(g [][]int) { RotateColsInStack(g, rng.Intn(bs), 1+rng.Intn(bs-1)) },
			"bands":       func(g [][]int) { RotateBands(g, 1+rng.Intn(bs-1)) },
			"stacks":      func(g [][]int) { RotateStacks(g, 1+rng.Intn(bs-1)) },
		}
		for name, apply := range transforms {
			grid := canonical(t, size)
			for i := 0; i < 200; i++ {
				apply(grid)
				if !model.IsValidSolution(grid) {
					t.Fatalf("%s on %dx%d broke the grid (iteration %d)", name, size, size, i)
				}
			}
		}
	}
}

func TestRotationsActuallyMoveThings(t *testing.T) {
	orig := canonical(t, 9)
	grid := model.CopyGrid(orig)
	RotateRowsInBand(grid, 0, 1)
	if grid[0][0] != orig[1][0] || grid[2][0] != orig[0][0] {
		t.Fatal("RotateRowsInBand: row 0 should now be old row 1, row 2 old row 0")
	}
	if grid[3][0] != orig[3][0] {
		t.Fatal("RotateRowsInBand touched another band")
	}
	grid = model.CopyGrid(orig)
	RotateStacks(grid, 1)
	if grid[0][0] != orig[0][3] || grid[0][6] != orig[0][0] {
		t.Fatal("RotateStacks moved the wrong columns")
	}
}

func TestFullPipelineManySeeds(t *testing.T) {
	runs := map[int]int{4: 10000, 9: 10000, 16: 2000}
	for size, n := range runs {
		shuffler := NewRotationShuffler(randutil.NewLocked(int64(size)))
		base := canonical(t, size)
		for i := 0; i < n; i++ {
			grid := model.CopyGrid(base)
			shuffler.Shuffle(grid)
			if !model.IsValidSolution(grid) {
				t.Fatalf("pipeline broke a %dx%d grid on run %d", size, size, i)
			}
		}
	}
}

func newGenerator(t *testing.T, seed int64) *RandomRemovalGenerator {
	t.Helper()
	rng := randutil.NewLocked(seed)
	repo := repository.NewInMemoryBaseBoardRepo(rng)
	if err := repository.SeedDefaults(repo); err != nil {
		t.Fatal(err)
	}
	return NewRandomRemovalGenerator(repo, NewRotationShuffler(rng), rng)
}

func TestGeneratorRespectsDifficultyBands(t *testing.T) {
	gen := newGenerator(t, 42)
	for _, size := range boardSizes {
		for _, d := range []model.Difficulty{model.Easy, model.Medium, model.Hard} {
			min, max, _ := d.ClueRange(size * size)
			for i := 0; i < 50; i++ {
				b, err := gen.Generate(size, d)
				if err != nil {
					t.Fatal(err)
				}
				if b.Size() != size || b.BoxSize()*b.BoxSize() != size {
					t.Fatalf("Generate(%d) → size %d box %d", size, b.Size(), b.BoxSize())
				}
				clues := 0
				view := b.Snapshot()
				for r := 0; r < size; r++ {
					for c := 0; c < size; c++ {
						filled := view.Grid[r][c] != 0
						if filled != view.Fixed[r][c] {
							t.Fatalf("(%d,%d): filled=%v fixed=%v", r, c, filled, view.Fixed[r][c])
						}
						if filled {
							clues++
						}
					}
				}
				if clues < min || clues > max {
					t.Fatalf("%s %dx%d: %d clues outside [%d,%d]", d, size, size, clues, min, max)
				}
			}
		}
	}
}

func TestGeneratorUnsupportedSize(t *testing.T) {
	gen := newGenerator(t, 1)
	for _, size := range []int{0, 1, 6, 8, 25} { // 25 is square but has no seed
		if _, err := gen.Generate(size, model.Easy); err == nil {
			t.Errorf("Generate(%d) should fail", size)
		}
	}
}
