package controller

import (
	"time"

	"jiostar/sudoku/internal/model"
)

// POST /users
type CreateUserRequest struct {
	Name string `json:"name"`
}

type UserResponse struct {
	UserID string `json:"userId"`
	Name   string `json:"name"`
}

// POST /games
type InitGameRequest struct {
	UserID     string `json:"userId"`
	Difficulty string `json:"difficulty"`          // EASY | MEDIUM | HARD (case-insensitive)
	BoardSize  int    `json:"boardSize,omitempty"` // 4, 9, 16, ... (0 → default 9); boxes are √size × √size
}

// POST /games/{gameId}/moves
type MoveRequest struct {
	GameID string `json:"-"` // from the path
	UserID string `json:"userId"`
	Row    int    `json:"row"`
	Col    int    `json:"col"`
	Value  int    `json:"value"` // 1..size, or 0 to clear
}

type GameResponse struct {
	GameID     string            `json:"gameId"`
	Difficulty model.Difficulty  `json:"difficulty"`
	Status     model.GameStatus  `json:"status"`
	MoveCount  int               `json:"moveCount"`
	Board      model.BoardView   `json:"board"`
	LastMove   *model.MoveResult `json:"lastMove,omitempty"` // only on MakeMove
}

// GET /users/{userId}/games
type GameSummary struct {
	GameID     string           `json:"gameId"`
	Difficulty model.Difficulty `json:"difficulty"`
	Size       int              `json:"size"`
	Status     model.GameStatus `json:"status"`
	CreatedAt  time.Time        `json:"createdAt"`
}
