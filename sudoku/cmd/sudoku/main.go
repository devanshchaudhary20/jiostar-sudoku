// Command sudoku is a demo driver that exercises every API of the
// SudokuController: create a user, start games of different sizes and
// difficulties, make valid / conflicting / rejected moves, and play a game
// through to COMPLETED.
package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"jiostar/sudoku/internal/app"
	"jiostar/sudoku/internal/controller"
	"jiostar/sudoku/internal/model"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	ctrl, err := app.New(time.Now().UnixNano())
	if err != nil {
		return err
	}

	user, err := ctrl.CreateUser(controller.CreateUserRequest{Name: "Devansh"})
	if err != nil {
		return err
	}
	fmt.Printf("Created user %s (%s)\n\n", user.UserID, user.Name)

	// 1. Start a 9x9 medium game.
	game, err := ctrl.InitGame(controller.InitGameRequest{UserID: user.UserID, Difficulty: "medium"})
	if err != nil {
		return err
	}
	fmt.Printf("== %s: %s %dx%d, %d clues ==\n", game.GameID, game.Difficulty, game.Board.Size, game.Board.Size, countFilled(game.Board))
	fmt.Println(render(game.Board))

	empty := firstEmpty(game.Board)
	fixed := firstFixed(game.Board)

	// 2. A conflicting move: copy a given from the same row. It is placed and flagged.
	clash := givenInRow(game.Board, empty.Row)
	resp, err := ctrl.MakeMove(controller.MoveRequest{GameID: game.GameID, UserID: user.UserID, Row: empty.Row, Col: empty.Col, Value: clash})
	if err != nil {
		return err
	}
	fmt.Printf("Move %d at (%d,%d): valid=%v conflicts=%v\n", clash, empty.Row, empty.Col, resp.LastMove.Valid, resp.LastMove.Conflicts)
	fmt.Println(render(resp.Board))

	// 3. Rejected requests: nothing is placed.
	tryReject(ctrl, "write to a prefilled cell", controller.MoveRequest{GameID: game.GameID, UserID: user.UserID, Row: fixed.Row, Col: fixed.Col, Value: 1})
	tryReject(ctrl, "value outside 1-9", controller.MoveRequest{GameID: game.GameID, UserID: user.UserID, Row: empty.Row, Col: empty.Col, Value: 10})
	tryReject(ctrl, "row out of bounds", controller.MoveRequest{GameID: game.GameID, UserID: user.UserID, Row: 9, Col: 0, Value: 1})
	tryReject(ctrl, "another user's game", controller.MoveRequest{GameID: game.GameID, UserID: "user-999", Row: empty.Row, Col: empty.Col, Value: 1})

	// 4. Undo by clearing (value 0): the flag disappears.
	resp, err = ctrl.MakeMove(controller.MoveRequest{GameID: game.GameID, UserID: user.UserID, Row: empty.Row, Col: empty.Col, Value: 0})
	if err != nil {
		return err
	}
	fmt.Printf("\nCleared (%d,%d): valid=%v, any conflicts left=%v\n\n", empty.Row, empty.Col, resp.LastMove.Valid, anyInvalid(resp.Board))

	// 5. Play the rest of the game with a solver and watch it complete.
	resp, err = solveThroughAPI(ctrl, user.UserID, resp)
	if err != nil {
		return err
	}
	fmt.Printf("Solved %s in %d moves → status %s\n", resp.GameID, resp.MoveCount, resp.Status)
	fmt.Println(render(resp.Board))
	tryReject(ctrl, "move after completion", controller.MoveRequest{GameID: game.GameID, UserID: user.UserID, Row: 0, Col: 0, Value: 1})

	// 6. Same user, more games at once, including another board size.
	hard, err := ctrl.InitGame(controller.InitGameRequest{UserID: user.UserID, Difficulty: "HARD"})
	if err != nil {
		return err
	}
	small, err := ctrl.InitGame(controller.InitGameRequest{UserID: user.UserID, Difficulty: "easy", BoardSize: 4})
	if err != nil {
		return err
	}
	fmt.Printf("\n== %s: HARD 9x9, %d clues ==\n", hard.GameID, countFilled(hard.Board))
	fmt.Println(render(hard.Board))
	fmt.Printf("== %s: EASY 4x4 (four 2x2 boxes), %d clues ==\n", small.GameID, countFilled(small.Board))
	fmt.Println(render(small.Board))

	tryInit(ctrl, "board size 6 (not a perfect square)", controller.InitGameRequest{UserID: user.UserID, Difficulty: "easy", BoardSize: 6})
	tryInit(ctrl, "board size 25 (no base board seeded)", controller.InitGameRequest{UserID: user.UserID, Difficulty: "easy", BoardSize: 25})
	fmt.Println()

	games, err := ctrl.ListGames(user.UserID)
	if err != nil {
		return err
	}
	fmt.Printf("Games for %s:\n", user.UserID)
	for _, g := range games {
		fmt.Printf("  %-8s %-6s %2dx%-2d %s\n", g.GameID, g.Difficulty, g.Size, g.Size, g.Status)
	}
	return nil
}

func tryReject(ctrl *controller.SudokuController, label string, req controller.MoveRequest) {
	_, err := ctrl.MakeMove(req)
	if err == nil {
		fmt.Printf("  %-26s → unexpectedly accepted\n", label)
		return
	}
	fmt.Printf("  %-26s → rejected: %v\n", label, err)
}

func tryInit(ctrl *controller.SudokuController, label string, req controller.InitGameRequest) {
	if _, err := ctrl.InitGame(req); err != nil {
		fmt.Printf("  %-38s → rejected: %v\n", label, err)
		return
	}
	fmt.Printf("  %-38s → unexpectedly accepted\n", label)
}

// solveThroughAPI solves the current board and submits every missing value
// as a real move, so the completion logic is exercised end to end.
func solveThroughAPI(ctrl *controller.SudokuController, userID string, resp controller.GameResponse) (controller.GameResponse, error) {
	grid := model.CopyGrid(resp.Board.Grid)
	if !solve(grid, resp.Board.BoxSize) { // BoxSize in the view is the derived √size
		return resp, errors.New("puzzle has no solution")
	}
	var err error
	for r := range grid {
		for c := range grid[r] {
			if resp.Board.Grid[r][c] != 0 {
				continue
			}
			resp, err = ctrl.MakeMove(controller.MoveRequest{GameID: resp.GameID, UserID: userID, Row: r, Col: c, Value: grid[r][c]})
			if err != nil {
				return resp, err
			}
		}
	}
	return resp, nil
}

// solve is a plain backtracking solver used only by this demo.
func solve(grid [][]int, boxSize int) bool {
	size := len(grid)
	for r := 0; r < size; r++ {
		for c := 0; c < size; c++ {
			if grid[r][c] != 0 {
				continue
			}
			for v := 1; v <= size; v++ {
				if canPlace(grid, boxSize, r, c, v) {
					grid[r][c] = v
					if solve(grid, boxSize) {
						return true
					}
					grid[r][c] = 0
				}
			}
			return false
		}
	}
	return true
}

func canPlace(grid [][]int, boxSize, r, c, v int) bool {
	r0, c0 := r/boxSize*boxSize, c/boxSize*boxSize
	for i := 0; i < len(grid); i++ {
		if grid[r][i] == v || grid[i][c] == v || grid[r0+i/boxSize][c0+i%boxSize] == v {
			return false
		}
	}
	return true
}

// render draws the board. Given digits are plain, player entries are
// wrapped in [ ] and conflicting entries are marked with !.
func render(b model.BoardView) string {
	w := len(strconv.Itoa(b.Size)) + 3
	var sb strings.Builder
	sep := func() {
		for s := 0; s < b.BoxSize; s++ {
			sb.WriteString("+" + strings.Repeat("-", w*b.BoxSize+1))
		}
		sb.WriteString("+\n")
	}
	for r := 0; r < b.Size; r++ {
		if r%b.BoxSize == 0 {
			sep()
		}
		for c := 0; c < b.Size; c++ {
			if c%b.BoxSize == 0 {
				sb.WriteString("| ")
			}
			cell := "."
			switch v := b.Grid[r][c]; {
			case v == 0:
			case b.Fixed[r][c]:
				cell = strconv.Itoa(v)
			case b.Invalid[r][c]:
				cell = strconv.Itoa(v) + "!"
			default:
				cell = "[" + strconv.Itoa(v) + "]"
			}
			sb.WriteString(fmt.Sprintf("%-*s", w, cell))
		}
		sb.WriteString("|\n")
	}
	sep()
	return sb.String()
}

func countFilled(b model.BoardView) int {
	n := 0
	for _, row := range b.Grid {
		for _, v := range row {
			if v != 0 {
				n++
			}
		}
	}
	return n
}

func anyInvalid(b model.BoardView) bool {
	for _, row := range b.Invalid {
		for _, inv := range row {
			if inv {
				return true
			}
		}
	}
	return false
}

func firstEmpty(b model.BoardView) model.Position {
	for r, row := range b.Grid {
		for c, v := range row {
			if v == 0 {
				return model.Position{Row: r, Col: c}
			}
		}
	}
	return model.Position{}
}

func firstFixed(b model.BoardView) model.Position {
	for r, row := range b.Fixed {
		for c, f := range row {
			if f {
				return model.Position{Row: r, Col: c}
			}
		}
	}
	return model.Position{}
}

// givenInRow returns any given in row r, or failing that any given on the board.
func givenInRow(b model.BoardView, r int) int {
	for c, v := range b.Grid[r] {
		if b.Fixed[r][c] {
			return v
		}
	}
	p := firstFixed(b)
	return b.Grid[p.Row][p.Col]
}
