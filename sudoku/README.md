# Sudoku (Go, in-memory LLD)

An online Sudoku game service, implemented as a machine-coding / LLD exercise.
The full design is in `../DESIGN.md`.

## Requirements

Go 1.24 or newer. There are no third-party dependencies.

## Run

Run every command from this `sudoku` folder, where `go.mod` is.

```bash
go run ./cmd/server                 # HTTP server on http://localhost:8080
PORT=9000 go run ./cmd/server       # ...or on another port
go run ./cmd/sudoku                 # console demo: every API, a conflict, rejections, a full solve
go test -race ./...                 # all tests with the race detector
go build -o sudoku-server ./cmd/server   # standalone binary
```

Stop the server with Ctrl+C. It finishes any requests still in flight before exiting. All data is in memory, so it's gone once the server stops.

## Try it with curl

```bash
curl -X POST localhost:8080/users -d '{"name":"devansh"}'
# → {"userId":"user-1","name":"devansh"}

curl -X POST localhost:8080/games -d '{"userId":"user-1","difficulty":"medium","boardSize":9}'
# → {"gameId":"game-2", "status":"IN_PROGRESS", "board":{...}, ...}

curl -X POST localhost:8080/games/game-2/moves -d '{"userId":"user-1","row":0,"col":2,"value":5}'
# → the whole board, plus "lastMove":{"valid":true|false,"conflicts":[...]}

curl "localhost:8080/games/game-2?userId=user-1"
curl localhost:8080/users/user-1/games
curl localhost:8080/healthz
```

### Status codes

| Code | When |
|---|---|
| 200 / 201 | Success. A **conflicting** move is still a success: it returns 200 with `lastMove.valid: false`. |
| 400 | Malformed JSON, an unknown field, or a missing `row`, `col` or `value`. Also an empty name, a bad difficulty, a board size that isn't a perfect square, a value outside 1..size, or a position off the board. |
| 403 | The game belongs to another user |
| 404 | Unknown user or game |
| 409 | Writing to a prefilled cell, or moving on a completed game |

Errors come back as `{"error": "..."}`.

## Layout

```
internal/
  model/       Position, Cell, Board, Difficulty, Game, User, MoveValidator interface
  validator/   StandardValidator (row / column / box rules)
  generator/   RotationShuffler + RandomRemovalGenerator
  repository/  in-memory users, games, base boards (+ seed grids)
  controller/  SudokuController (the API surface) + DTOs
  httpapi/     net/http handlers: JSON in/out, error → status mapping
  app/         wires everything together
  errs/        sentinel errors
  randutil/    goroutine-safe random source
cmd/server/    HTTP server (port 8080, or $PORT)
cmd/sudoku/    console demo driver
```

## API (controller method → REST)

| Method | REST | Body |
|---|---|---|
| `CreateUser` | `POST /users` | `{name}` |
| `InitGame` | `POST /games` | `{userId, difficulty, boardSize?}` (boardSize defaults to 9) |
| `MakeMove` | `POST /games/{gameId}/moves` | `{userId, row, col, value}` (value is 1..size, or 0 to clear) |
| `GetGame` | `GET /games/{gameId}?userId=` | none |
| `ListGames` | `GET /users/{userId}/games` | none |

Every move returns the whole board: `grid`, `fixed` and `invalid`, plus
`status` and `lastMove {valid, conflicts}`.

## Rules implemented

- **Board and box size.** You choose the board size (4, 9, 16, …). The box size is always √size, so 9×9 has nine 3×3 boxes and 16×16 has sixteen 4×4 boxes. Sizes that aren't perfect squares are rejected.
- **Difficulty.** Prefilled cells are a fraction of the total. On 9×9, Hard is 22–26, Medium is 27–54 (inclusive) and Easy is 55–64.
- **Conflicting values.** These are placed and flagged, not rejected.
- **Rejected requests.** Out-of-bounds positions, values outside 1..size, prefilled cells, someone else's game and finished games are rejected and change nothing.
- **Overwriting.** Overwriting a cell, or clearing it with 0, is the undo.
- **Completion.** The game is `COMPLETED` when the board is full and has no conflicts.

## Adding things later

- **A new board size.** Add a solved grid of that size with `BaseBoardRepository.Add`, or use `repository.CanonicalGrid(size)`.
- **A sudoku variant.** Write a new `MoveValidator` that returns extra units, then pass it to `NewSudokuController`.
- **Authentication.** `userId` currently comes from the request. Replace it with an identity from a token in `httpapi`; the controller doesn't change.
- **Persistence.** Swap the in-memory repositories for DB-backed ones that implement the same interfaces.
