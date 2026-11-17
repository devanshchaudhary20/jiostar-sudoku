package controller_test

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"testing"

	"jiostar/sudoku/internal/app"
	"jiostar/sudoku/internal/controller"
	"jiostar/sudoku/internal/errs"
	"jiostar/sudoku/internal/generator"
	"jiostar/sudoku/internal/randutil"
	"jiostar/sudoku/internal/repository"
	"jiostar/sudoku/internal/validator"
)

func setup(t *testing.T) (*controller.SudokuController, string) {
	t.Helper()
	ctrl, err := app.New(7)
	if err != nil {
		t.Fatal(err)
	}
	u, err := ctrl.CreateUser(controller.CreateUserRequest{Name: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	return ctrl, u.UserID
}

func TestCreateUserValidation(t *testing.T) {
	ctrl, _ := setup(t)
	if _, err := ctrl.CreateUser(controller.CreateUserRequest{Name: "  "}); !errors.Is(err, errs.ErrInvalidName) {
		t.Fatalf("got %v", err)
	}
}

func TestInitGameErrors(t *testing.T) {
	ctrl, uid := setup(t)
	cases := []struct {
		req  controller.InitGameRequest
		want error
	}{
		{controller.InitGameRequest{UserID: "nobody", Difficulty: "easy"}, errs.ErrUserNotFound},
		{controller.InitGameRequest{UserID: uid, Difficulty: "extreme"}, errs.ErrInvalidDifficulty},
		{controller.InitGameRequest{UserID: uid, Difficulty: "easy", BoardSize: 1}, errs.ErrUnsupportedSize},
		{controller.InitGameRequest{UserID: uid, Difficulty: "easy", BoardSize: 6}, errs.ErrUnsupportedSize},  // not a perfect square
		{controller.InitGameRequest{UserID: uid, Difficulty: "easy", BoardSize: 8}, errs.ErrUnsupportedSize},  // not a perfect square
		{controller.InitGameRequest{UserID: uid, Difficulty: "easy", BoardSize: 25}, errs.ErrUnsupportedSize}, // valid size, no seed
	}
	for _, tc := range cases {
		if _, err := ctrl.InitGame(tc.req); !errors.Is(err, tc.want) {
			t.Errorf("%+v → %v, want %v", tc.req, err, tc.want)
		}
	}
}

func TestDefaultSizeAndMultipleGamesPerUser(t *testing.T) {
	ctrl, uid := setup(t)
	g1, err := ctrl.InitGame(controller.InitGameRequest{UserID: uid, Difficulty: "hard"})
	if err != nil {
		t.Fatal(err)
	}
	if g1.Board.Size != 9 {
		t.Fatalf("default size %d, want 9", g1.Board.Size)
	}
	g2, err := ctrl.InitGame(controller.InitGameRequest{UserID: uid, Difficulty: "easy", BoardSize: 16})
	if err != nil {
		t.Fatal(err)
	}
	if g2.Board.Size != 16 || g2.Board.BoxSize != 4 || g1.Board.BoxSize != 3 || g1.GameID == g2.GameID {
		t.Fatal("second game wrong")
	}
	list, err := ctrl.ListGames(uid)
	if err != nil || len(list) != 2 {
		t.Fatalf("ListGames = %v, %v", list, err)
	}
}

func TestOwnershipAndUnknownGame(t *testing.T) {
	ctrl, alice := setup(t)
	bob, _ := ctrl.CreateUser(controller.CreateUserRequest{Name: "bob"})
	g, _ := ctrl.InitGame(controller.InitGameRequest{UserID: alice, Difficulty: "medium"})

	if _, err := ctrl.MakeMove(controller.MoveRequest{GameID: g.GameID, UserID: bob.UserID, Value: 1}); !errors.Is(err, errs.ErrNotGameOwner) {
		t.Fatalf("got %v, want ErrNotGameOwner", err)
	}
	if _, err := ctrl.GetGame(bob.UserID, g.GameID); !errors.Is(err, errs.ErrNotGameOwner) {
		t.Fatalf("got %v, want ErrNotGameOwner", err)
	}
	if _, err := ctrl.MakeMove(controller.MoveRequest{GameID: "game-404", UserID: alice, Value: 1}); !errors.Is(err, errs.ErrGameNotFound) {
		t.Fatalf("got %v, want ErrGameNotFound", err)
	}
}

func TestMoveReturnsWholeBoard(t *testing.T) {
	ctrl, uid := setup(t)
	g, _ := ctrl.InitGame(controller.InitGameRequest{UserID: uid, Difficulty: "easy"})
	r, c := emptyCell(g)
	resp, err := ctrl.MakeMove(controller.MoveRequest{GameID: g.GameID, UserID: uid, Row: r, Col: c, Value: 1})
	if err != nil {
		t.Fatal(err)
	}
	if resp.LastMove == nil || len(resp.Board.Grid) != 9 || resp.Board.Grid[r][c] != 1 || resp.MoveCount != 1 {
		t.Fatalf("unexpected response %+v", resp)
	}
}

// 100 goroutines fire random moves at the same game. Run with -race.
// Afterwards every flag and counter must match a fresh recount.
func TestConcurrentMovesOnOneGame(t *testing.T) {
	rng := randutil.NewLocked(11)
	base := repository.NewInMemoryBaseBoardRepo(rng)
	if err := repository.SeedDefaults(base); err != nil {
		t.Fatal(err)
	}
	games := repository.NewInMemoryGameRepo()
	ctrl := controller.NewSudokuController(
		repository.NewInMemoryUserRepo(), games,
		generator.NewRandomRemovalGenerator(base, generator.NewRotationShuffler(rng), rng),
		validator.NewStandardValidator(), &controller.SequentialIDGenerator{},
	)
	u, _ := ctrl.CreateUser(controller.CreateUserRequest{Name: "racer"})
	uid := u.UserID
	g, err := ctrl.InitGame(controller.InitGameRequest{UserID: uid, Difficulty: "hard"})
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for w := 0; w < 100; w++ {
		wg.Add(1)
		go func(seed int64) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(seed))
			for i := 0; i < 50; i++ {
				_, _ = ctrl.MakeMove(controller.MoveRequest{
					GameID: g.GameID, UserID: uid,
					Row: rng.Intn(9), Col: rng.Intn(9), Value: rng.Intn(10),
				})
				if i%10 == 0 {
					_, _ = ctrl.GetGame(uid, g.GameID)
				}
			}
		}(int64(w))
	}
	wg.Wait()

	game, err := games.Get(g.GameID)
	if err != nil {
		t.Fatal(err)
	}
	if err := game.Verify(); err != nil {
		t.Fatal(err)
	}
}

// Many users create games in parallel. Run with -race.
func TestConcurrentUsersCreatingGames(t *testing.T) {
	ctrl, err := app.New(3)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errCh := make(chan error, 50)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			u, err := ctrl.CreateUser(controller.CreateUserRequest{Name: fmt.Sprintf("u%d", i)})
			if err != nil {
				errCh <- err
				return
			}
			for _, d := range []string{"easy", "medium", "hard"} {
				if _, err := ctrl.InitGame(controller.InitGameRequest{UserID: u.UserID, Difficulty: d}); err != nil {
					errCh <- err
					return
				}
			}
			if list, _ := ctrl.ListGames(u.UserID); len(list) != 3 {
				errCh <- fmt.Errorf("user %s has %d games", u.UserID, len(list))
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
}

func emptyCell(g controller.GameResponse) (int, int) {
	for r, row := range g.Board.Grid {
		for c, v := range row {
			if v == 0 {
				return r, c
			}
		}
	}
	return -1, -1
}
