// Package httpapi exposes SudokuController over HTTP/JSON. Each handler is
// a thin adapter: decode the request, call the controller, encode the result.
// All game logic stays in the controller and below.
package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"jiostar/sudoku/internal/controller"
	"jiostar/sudoku/internal/errs"
)

const maxBodyBytes = 1 << 20 // 1 MiB

// NewHandler returns the router for the Sudoku API.
//
//	POST /users                      create a user
//	GET  /users/{userId}/games       list a user's games
//	POST /games                      start a game
//	GET  /games/{gameId}?userId=     get a game
//	POST /games/{gameId}/moves       make a move (value 0 clears)
//	GET  /healthz                    liveness check
func NewHandler(ctrl *controller.SudokuController, logger *log.Logger) http.Handler {
	h := &handler{ctrl: ctrl}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /users", h.createUser)
	mux.HandleFunc("GET /users/{userId}/games", h.listGames)
	mux.HandleFunc("POST /games", h.initGame)
	mux.HandleFunc("GET /games/{gameId}", h.getGame)
	mux.HandleFunc("POST /games/{gameId}/moves", h.makeMove)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	if logger == nil {
		return mux
	}
	return logRequests(mux, logger)
}

type handler struct {
	ctrl *controller.SudokuController
}

func (h *handler) createUser(w http.ResponseWriter, r *http.Request) {
	var req controller.CreateUserRequest
	if !decode(w, r, &req) {
		return
	}
	resp, err := h.ctrl.CreateUser(req)
	respond(w, http.StatusCreated, resp, err)
}

func (h *handler) listGames(w http.ResponseWriter, r *http.Request) {
	games, err := h.ctrl.ListGames(r.PathValue("userId"))
	respond(w, http.StatusOK, games, err)
}

func (h *handler) initGame(w http.ResponseWriter, r *http.Request) {
	var req controller.InitGameRequest
	if !decode(w, r, &req) {
		return
	}
	resp, err := h.ctrl.InitGame(req)
	respond(w, http.StatusCreated, resp, err)
}

func (h *handler) getGame(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("userId")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "query parameter userId is required")
		return
	}
	resp, err := h.ctrl.GetGame(userID, r.PathValue("gameId"))
	respond(w, http.StatusOK, resp, err)
}

// moveBody uses pointers so a missing field is an error rather than a silent
// zero. Without this, forgetting "value" would clear the cell.
type moveBody struct {
	UserID string `json:"userId"`
	Row    *int   `json:"row"`
	Col    *int   `json:"col"`
	Value  *int   `json:"value"`
}

func (h *handler) makeMove(w http.ResponseWriter, r *http.Request) {
	var body moveBody
	if !decode(w, r, &body) {
		return
	}
	if body.UserID == "" || body.Row == nil || body.Col == nil || body.Value == nil {
		writeError(w, http.StatusBadRequest, "userId, row, col and value are all required (value 0 clears a cell)")
		return
	}
	resp, err := h.ctrl.MakeMove(controller.MoveRequest{
		GameID: r.PathValue("gameId"),
		UserID: body.UserID,
		Row:    *body.Row,
		Col:    *body.Col,
		Value:  *body.Value,
	})
	respond(w, http.StatusOK, resp, err)
}

// decode reads a JSON body strictly: unknown fields, trailing data and
// oversized bodies are rejected with 400.
func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return false
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid JSON body: unexpected data after the JSON object")
		return false
	}
	return true
}

func respond(w http.ResponseWriter, okStatus int, body any, err error) {
	if err != nil {
		writeError(w, statusFor(err), err.Error())
		return
	}
	writeJSON(w, okStatus, body)
}

// statusFor maps service errors to HTTP status codes.
// Note: a conflicting move is not an error; it returns 200 with lastMove.valid=false.
func statusFor(err error) int {
	switch {
	case errors.Is(err, errs.ErrUserNotFound), errors.Is(err, errs.ErrGameNotFound):
		return http.StatusNotFound
	case errors.Is(err, errs.ErrNotGameOwner):
		return http.StatusForbidden
	case errors.Is(err, errs.ErrGameCompleted), errors.Is(err, errs.ErrFixedCell):
		return http.StatusConflict
	case errors.Is(err, errs.ErrOutOfBounds),
		errors.Is(err, errs.ErrInvalidValue),
		errors.Is(err, errs.ErrInvalidDifficulty),
		errors.Is(err, errs.ErrUnsupportedSize),
		errors.Is(err, errs.ErrInvalidName):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

type errorBody struct {
	Error string `json:"error"`
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorBody{Error: msg})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// logRequests logs one line per request: method, path, status, duration.
func logRequests(next http.Handler, logger *log.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		logger.Printf("%s %s → %d (%s)", r.Method, r.URL.RequestURI(), rec.status, fmt.Sprint(time.Since(start).Round(time.Microsecond)))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}
