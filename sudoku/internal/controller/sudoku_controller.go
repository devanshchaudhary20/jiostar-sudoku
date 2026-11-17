// Package controller is the API surface of the service. Each method maps
// one-to-one onto a REST endpoint (see the comment on each method).
package controller

import (
	"fmt"
	"strings"
	"time"

	"jiostar/sudoku/internal/errs"
	"jiostar/sudoku/internal/generator"
	"jiostar/sudoku/internal/model"
	"jiostar/sudoku/internal/repository"
)

// DefaultBoardSize is used when InitGame doesn't specify one: a 9x9 board
// with nine 3x3 boxes.
const DefaultBoardSize = 9

type SudokuController struct {
	users     repository.UserRepository
	games     repository.GameRepository
	generator generator.PuzzleGenerator
	validator model.MoveValidator // injected into every new game
	ids       IDGenerator
	now       func() time.Time
}

func NewSudokuController(
	users repository.UserRepository,
	games repository.GameRepository,
	gen generator.PuzzleGenerator,
	validator model.MoveValidator,
	ids IDGenerator,
) *SudokuController {
	return &SudokuController{
		users:     users,
		games:     games,
		generator: gen,
		validator: validator,
		ids:       ids,
		now:       time.Now,
	}
}

// CreateUser handles POST /users.
func (c *SudokuController) CreateUser(req CreateUserRequest) (UserResponse, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return UserResponse{}, errs.ErrInvalidName
	}
	u := &model.User{ID: c.ids.Next("user"), Name: name}
	if err := c.users.Save(u); err != nil {
		return UserResponse{}, err
	}
	return UserResponse{UserID: u.ID, Name: u.Name}, nil
}

// InitGame handles POST /games. It generates a new puzzle for the user.
// A user may have any number of games running at once.
func (c *SudokuController) InitGame(req InitGameRequest) (GameResponse, error) {
	if _, err := c.users.Get(req.UserID); err != nil {
		return GameResponse{}, err
	}
	d, err := model.ParseDifficulty(req.Difficulty)
	if err != nil {
		return GameResponse{}, err
	}
	size := req.BoardSize
	if size == 0 {
		size = DefaultBoardSize
	}
	// Box size is derived from the board size (√size); reject non-squares early.
	if _, err := model.BoxSizeFor(size); err != nil {
		return GameResponse{}, err
	}

	board, err := c.generator.Generate(size, d)
	if err != nil {
		return GameResponse{}, err
	}
	game := model.NewGame(c.ids.Next("game"), req.UserID, d, board, c.validator, c.now())
	if err := c.games.Save(game); err != nil {
		return GameResponse{}, err
	}
	return toResponse(game, game.State(), nil), nil
}

// MakeMove handles POST /games/{gameId}/moves.
//
// Lock order: the repository lock is taken and released inside games.Get,
// then the game lock inside ApplyMove. They are never held together.
func (c *SudokuController) MakeMove(req MoveRequest) (GameResponse, error) {
	game, err := c.ownedGame(req.UserID, req.GameID)
	if err != nil {
		return GameResponse{}, err
	}
	result, state, err := game.ApplyMove(model.Position{Row: req.Row, Col: req.Col}, req.Value)
	if err != nil {
		return GameResponse{}, err
	}
	return toResponse(game, state, &result), nil
}

// GetGame handles GET /games/{gameId}?userId=.
func (c *SudokuController) GetGame(userID, gameID string) (GameResponse, error) {
	game, err := c.ownedGame(userID, gameID)
	if err != nil {
		return GameResponse{}, err
	}
	return toResponse(game, game.State(), nil), nil
}

// ListGames handles GET /users/{userId}/games.
func (c *SudokuController) ListGames(userID string) ([]GameSummary, error) {
	if _, err := c.users.Get(userID); err != nil {
		return nil, err
	}
	games, err := c.games.ListByUser(userID)
	if err != nil {
		return nil, err
	}
	out := make([]GameSummary, 0, len(games))
	for _, g := range games {
		state := g.State()
		out = append(out, GameSummary{
			GameID:     g.ID,
			Difficulty: g.Difficulty,
			Size:       state.Board.Size,
			Status:     state.Status,
			CreatedAt:  g.CreatedAt,
		})
	}
	return out, nil
}

// ownedGame fetches a game and checks it belongs to userID.
// Game.UserID is immutable, so reading it needs no lock.
func (c *SudokuController) ownedGame(userID, gameID string) (*model.Game, error) {
	game, err := c.games.Get(gameID)
	if err != nil {
		return nil, err
	}
	if game.UserID != userID {
		return nil, fmt.Errorf("%w: %s", errs.ErrNotGameOwner, gameID)
	}
	return game, nil
}

func toResponse(g *model.Game, s model.GameState, last *model.MoveResult) GameResponse {
	return GameResponse{
		GameID:     g.ID,
		Difficulty: g.Difficulty,
		Status:     s.Status,
		MoveCount:  s.MoveCount,
		Board:      s.Board,
		LastMove:   last,
	}
}
