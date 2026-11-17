# Sudoku Game — Low-Level Design (Go)

## 1. Scope

A machine-coding/LLD exercise. The service is in-memory and is driven through `SudokuController` methods that map one-to-one onto REST endpoints.

**Functional requirements**
- Board size is configurable. It must be a perfect square: 4×4, 9×9, 16×16, and so on. The default is 9×9.
- Box size is derived from the board size. A `size×size` board has `size` boxes of `√size × √size`: 4×4 has four 2×2 boxes, 9×9 has nine 3×3 boxes and 16×16 has sixteen 4×4 boxes.
- There are always three difficulty levels: Easy, Medium and Hard. Each is defined as a fraction of the total cell count.
- Puzzles come from stored, fully solved base boards. Each base board goes through a fixed sequence of validity-preserving rotations (§6), then cells are removed according to the difficulty.
- There are three APIs: create a user, initialise a game, and make a move.
- A move counts as valid when the new value doesn't repeat within its row, column or box. Invalid moves are **placed and flagged**, not rejected.
- Players can overwrite or clear their own entries. Overwriting a cell is how a move is undone, so there is no separate undo API. Prefilled cells are locked.
- Entered numbers must be in `1..size`: 1–9 on 9×9, 1–4 on 4×4 and 1–16 on 16×16.
- A user can have many active games. Each game belongs to exactly one user.
- Every move response returns the whole board.

**Out of scope for v1**
- Unique-solution guarantee.
- Persistence.
- Authentication.
- Hints.

---

## 2. Key decisions

| Topic | Decision |
|---|---|
| Board size | Only the board size (`4`, `9`, `16`, …) is configured. The box size is always derived as `boxSize = √size`, by `model.BoxSizeFor(size)`, which rejects anything that isn't a perfect square ≥ 4. Callers never pass a box size. |
| Difficulty bands | Bands are fractions of `N = size²`. **Hard** is `[⌈0.27N⌉, ⌈N/3⌉−1]`. **Medium** is `[⌈N/3⌉, ⌊2N/3⌋]` and is inclusive. **Easy** is `[⌊2N/3⌋+1, ⌊0.8N⌋]`. |
| 9×9 bands | Hard is 22–26 clues, Medium is 27–54 and Easy is 55–64. |
| Clue count | A uniformly random value within the band. |
| Variety | A fixed, ordered pipeline of rotations is applied to every band and stack, plus digit relabelling (§6). Each step is proven to keep the grid a valid solution. |
| Validity | Only row, column and box constraints are checked, through `MoveValidator`. The board is never compared to the base solution, because removing cells can leave several valid solutions. |
| Invalid move | The value is placed and the cell is flagged `Invalid`. The response says the move conflicted and lists the conflicting positions. |
| Request errors | These are rejected with an error and never placed: a row or column out of bounds, a value outside `1..size`, a prefilled cell, a wrong owner, or a finished game. A value of `0` is the clear operation, not a number. |
| Undo | Not a feature. The player overwrites the cell, or clears it with `0`. |
| Completion | The status becomes `COMPLETED` when every cell is filled **and** no cell is flagged. A full board that still has conflicts stays `IN_PROGRESS`. |
| Concurrency | Each repository has an `RWMutex` that guards its maps. Each game has an `RWMutex` that guards its board. The two locks are never held together (§9). |
| Randomness | `*rand.Rand` is injected so tests can be deterministic. |

### Responsibilities

Each component has one job, and each can be extended on its own.

| Component | Job |
|---|---|
| `Cell` | State of one position: its value, whether it is fixed, and whether it is invalid. Future per-cell state such as pencil marks or a last-modified time goes here. |
| `Board` | Holds the grid and its counters. It knows nothing about the rules. |
| `MoveValidator` | Holds the rules: input checks, which constraint groups a cell belongs to, and which peers conflict with it. A sudoku variant such as diagonal or killer sudoku becomes a new implementation. |
| `Game` | Orchestrates a move. It validates the input, writes the value, re-flags affected cells and updates the status. |

---

## 3. Package layout

```
jiostar/sudoku/
├── go.mod
├── cmd/
│   └── sudoku/main.go            # demo driver: create user, init game, play moves
└── internal/
    ├── app/app.go                # wires the in-memory object graph
    ├── randutil/locked.go        # goroutine-safe random source
    ├── model/
    │   ├── position.go
    │   ├── difficulty.go         # Difficulty enum + clue-range calculation
    │   ├── cell.go
    │   ├── board.go              # grid + counters, no rules
    │   ├── move_validator.go     # MoveValidator interface (kept here to avoid an import cycle)
    │   ├── game.go               # Game + GameStatus, ApplyMove
    │   └── user.go
    ├── validator/
    │   └── standard_validator.go # row / column / box rules
    ├── generator/
    │   ├── puzzle_generator.go   # base board → rotated → cells removed
    │   └── shuffler.go           # ordered, validity-preserving rotations
    ├── repository/
    │   ├── base_board_repo.go    # solved grids keyed by size
    │   ├── game_repo.go
    │   └── user_repo.go
    ├── controller/
    │   ├── sudoku_controller.go
    │   └── dto.go                # request / response structs
    └── errs/
        └── errors.go
```

---

## 4. Entities

### 4.1 Difficulty

```go
type Difficulty string

const (
    Easy   Difficulty = "EASY"
    Medium Difficulty = "MEDIUM"
    Hard   Difficulty = "HARD"
)

// ClueRange returns the inclusive [min, max] number of prefilled cells
// for a board with totalCells = size*size.
func (d Difficulty) ClueRange(totalCells int) (min, max int, err error)
```

| Size | N | Hard | Medium | Easy |
|---|---|---|---|---|
| 4×4 | 16 | 5–5 | 6–10 | 11–12 |
| 9×9 | 81 | 22–26 | 27–54 | 55–64 |
| 16×16 | 256 | 70–85 | 86–170 | 171–204 |

### 4.2 Position and Cell

```go
type Position struct{ Row, Col int }

type Cell struct {
    Value   int  // 0 = empty, 1..size otherwise
    Fixed   bool // prefilled; cannot be changed
    Invalid bool // conflicts with another cell in one of its constraint groups
}
```

### 4.3 Board

`Board` is pure data. It keeps `filledCount` and `invalidCount` in step with every write, so the completion check is `O(1)`.

```go
type Board struct {
    size         int
    boxSize      int // derived: √size
    cells        [][]Cell
    filledCount  int
    invalidCount int
}

func NewBoard(size int) (*Board, error)           // size must be a perfect square ≥ 4
func NewBoardFromGrid(grid [][]int) (*Board, error) // size = len(grid); non-zero cells → Fixed

func (b *Board) Size() int
func (b *Board) BoxSize() int                    // √Size, derived
func (b *Board) InBounds(p Position) bool
func (b *Board) Cell(p Position) Cell
func (b *Board) SetValue(p Position, v int)      // adjusts filledCount
func (b *Board) SetInvalid(p Position, inv bool) // adjusts invalidCount
func (b *Board) IsFull() bool                    // filledCount == size*size
func (b *Board) HasConflicts() bool              // invalidCount > 0
func (b *Board) Snapshot() BoardView             // deep copy for responses
```

### 4.4 User

```go
type User struct {
    ID   string
    Name string
}
```

The user-to-games link lives in `GameRepository`, which indexes games by user ID.

---

## 5. MoveValidator

```go
// model/move_validator.go
type MoveValidator interface {
    // ValidateInput rejects requests that must never be placed.
    ValidateInput(b *Board, p Position, value int) error
    // Units returns every constraint group (row, column, box, ...) that contains p.
    Units(b *Board, p Position) [][]Position
    // Conflicts returns the peers of p that hold the same non-zero value.
    Conflicts(b *Board, p Position) []Position
}
```

`StandardValidator` implements the classic rules:

- **`ValidateInput`** checks, in order:
  - The position is out of bounds → `ErrOutOfBounds`.
  - The value is not `0` and is outside `1..size` → `ErrInvalidValue`.
  - The cell is fixed → `ErrFixedCell`.
- **`Units`** returns row `p.Row`, column `p.Col`, and the box with origin `(p.Row/boxSize*boxSize, p.Col/boxSize*boxSize)`.
- **`Conflicts`** collects the union of the cell's units without duplicates. It skips `p` and returns the cells whose value equals `p`'s non-zero value.

**Extending to a variant.** A diagonal variant only needs `Units` to also return the diagonals that contain `p`. `Conflicts` and the re-flagging in `Game` then pick up the new rule without any other change.

---

## 6. Puzzle generation

### 6.1 Why the board stays solvable

There are two steps, and each one is safe.

1. **Removing cells is always safe.** Blanking cells of a valid solved grid can't make the puzzle unsolvable, because the original grid still satisfies every constraint. Uniqueness isn't guaranteed, and that is out of scope.
2. **The shuffle is the only risk.** The rotation pipeline therefore uses only transforms that map a valid solved grid to another valid solved grid. Each transform is applied in a fixed order, to **every** band and stack.

Two terms are used below:

- A **band** is a horizontal strip of `boxSize` (= √size) rows, one row of boxes. On 9×9, rows 0–2, 3–5 and 6–8 form the three bands.
- A **stack** is the vertical equivalent, a strip of `boxSize` columns.

### 6.2 The pipeline

The `RotationShuffler` runs these steps in this order. Each step is listed with the reason it keeps the grid valid.

1. **Relabel digits.**
   - **What it does:** picks a random permutation `π` of `1..size` and replaces every value `v` with `π(v)`.
   - **Why it's valid:** `π` is a bijection, so any group of distinct values stays distinct.
2. **Rotate rows within each band.**
   - **What it does:** for each band `b = 0..boxSize−1`, cyclically shifts its `boxSize` rows by a random offset `k_b`.
   - **Why it's valid:** each row's contents are unchanged, so rows stay valid. Each column only reorders its values, so columns stay valid. Rows never leave their band, so each box keeps the same set of cells and stays valid.
3. **Rotate columns within each stack.**
   - **What it does:** for each stack `s`, cyclically shifts its columns by a random offset `k_s`.
   - **Why it's valid:** the same argument as step 2, with rows and columns swapped.
4. **Rotate bands.**
   - **What it does:** cyclically shifts the `boxSize` bands by a random offset.
   - **Why it's valid:** whole bands move, so each box moves intact. Rows are unchanged and columns are reordered.
5. **Rotate stacks.**
   - **What it does:** cyclically shifts the stacks by a random offset.
   - **Why it's valid:** the same argument as step 4.

A random offset of `0` is allowed, so any single step may leave the grid unchanged.

```go
type Shuffler interface {
    Shuffle(grid [][]int) // boxSize is derived from len(grid)
}

func (s *RotationShuffler) Shuffle(grid [][]int) {
    boxSize := √len(grid)
    s.relabelDigits(grid)
    for band := 0; band < boxSize; band++ {
        s.rotateRowsInBand(grid, band, s.rng.Intn(boxSize))
    }
    for stack := 0; stack < boxSize; stack++ {
        s.rotateColsInStack(grid, stack, s.rng.Intn(boxSize))
    }
    s.rotateBands(grid, s.rng.Intn(boxSize))
    s.rotateStacks(grid, s.rng.Intn(boxSize))
}
```

### 6.3 What is deliberately not allowed

- **Moving a row into another band**, for example swapping row 0 with row 3. Row 3 brings a different set of digits into box 0, and they can repeat digits already in rows 1–2 of that box.
- **Rotating the contents of a single box**, for example turning box 0 by 90°. The box itself stays valid, but the rows and columns that cross it get new values and can end up with duplicates.

### 6.4 Safety net and variety

- **Safety net.** After shuffling, the generator runs `isValidSolution(grid)`. A failure returns `ErrInvalidBaseBoard`. It shouldn't happen, but it turns any transform bug into a visible error instead of a broken game. `BaseBoardRepository.Add` runs the same check on every seed grid.
- **Variety.** On 9×9, one base board yields up to 9! × 3³ × 3³ × 3 × 3 ≈ **2.4 billion** distinct grids before any cells are removed.

### 6.5 Generator

```go
type PuzzleGenerator interface {
    Generate(size int, d Difficulty) (*Board, error) // size = 4, 9, 16, ...
}

type RandomRemovalGenerator struct {
    baseRepo BaseBoardRepository
    shuffler Shuffler
    rng      *rand.Rand
}
```

`Generate` runs these steps:

1. Check `size` is a perfect square, then get a random solved `size×size` grid from `baseRepo.GetRandom(size)`.
2. Deep-copy the grid, run `shuffler.Shuffle` on it, and then run `isValidSolution`.
3. Pick a clue count `k` uniformly from `d.ClueRange(size²)`.
4. Shuffle all `size²` cell indices and blank the first `size² − k` of them.
5. Call `NewBoardFromGrid`. The remaining values become `Fixed`.

---

## 7. Game

```go
type GameStatus string

const (
    InProgress GameStatus = "IN_PROGRESS"
    Completed  GameStatus = "COMPLETED"
)

type Game struct {
    mu         sync.RWMutex // see §9; Game is always handled as *Game, never copied
    ID         string
    UserID     string
    Difficulty Difficulty
    Board      *Board
    Status     GameStatus
    MoveCount  int
    CreatedAt  time.Time
    validator  MoveValidator
}

func (g *Game) ApplyMove(p Position, value int) (MoveResult, BoardView, error)
func (g *Game) View() BoardView // takes RLock

type MoveResult struct {
    Valid     bool       // false when the placed value conflicts
    Conflicts []Position // peers it clashes with
}
```

`ApplyMove` runs these steps:

1. `g.mu.Lock()`, followed by `defer g.mu.Unlock()`.
2. If the status is `Completed`, return `ErrGameCompleted`.
3. Call `validator.ValidateInput(board, p, value)` and return any error.
4. Call `board.SetValue(p, value)`.
5. **Re-flag.** Let `affected` be the union of `validator.Units(board, p)`, which includes `p`. For every non-fixed cell `q` in `affected`, call `board.SetInvalid(q, cell(q).Value != 0 && len(validator.Conflicts(board, q)) > 0)`.
   - **Why the union is enough:** changing `p` can only alter conflicts among cells that share a unit with `p`.
   - **Why calling `Conflicts(q)` on each cell is correct:** it checks *all* of `q`'s units. A cell that still clashes elsewhere stays flagged.
   - **Fixed cells are never flagged.** If a player entry clashes with a given, only the entry is flagged.
6. Increment `MoveCount`.
7. If `board.IsFull()` and `!board.HasConflicts()`, set the status to `Completed`.
8. Build the `MoveResult` from `Conflicts(p)`. Clearing a cell always returns `Valid: true`.
9. Take `board.Snapshot()` **inside the lock** and return it with the result.

Each move rechecks at most `3·size` cells, and each recheck costs `O(size)`.

---

## 8. Repositories (in-memory)

```go
type BaseBoardRepository interface {
    Add(grid [][]int) error              // rejects grids that aren't complete valid solutions
    GetRandom(size int) ([][]int, error) // returns a copy
}

type GameRepository interface {
    Save(g *Game) error
    Get(id string) (*Game, error)
    ListByUser(userID string) ([]*Game, error)
}

type UserRepository interface {
    Save(u *User) error
    Get(id string) (*User, error)
}
```

`InMemoryGameRepo` keeps two maps: `byID map[string]*Game` and `byUser map[string][]string`. Both are guarded by the same `RWMutex`.

The base board repository is seeded at startup with a few 9×9 grids and one 4×4 grid.

---

## 9. Concurrency

There are two independent layers of locking, and each protects something different.

### 9.1 Repository lock: one `sync.RWMutex` per repository

**What it protects:** the maps inside the repository. `GameRepository` has `byID` and `byUser`, and `UserRepository` has its user map.

**Why it's needed:** Go maps are not safe for concurrent use. A write that happens alongside any other read or write isn't just a data race; the runtime aborts the whole process with `fatal error: concurrent map writes`. Different users create games and register at the same time, so concurrent writes are normal traffic, not an edge case.

**Why it's an `RWMutex`:**

- Every `MakeMove` and `GetGame` call starts with `games.Get(id)`, so reads far outnumber writes.
- With `RLock`, all those lookups run in parallel.
- Only `Save`, which runs once per new game or user, takes the exclusive `Lock`.

**Scope:** the lock is held only for the map operation itself. `Get` returns the `*Game` pointer and releases the lock straight away. The lock doesn't, and shouldn't, cover what happens to the game afterwards.

### 9.2 Game lock: one `sync.RWMutex` per `Game`

**What it protects:** everything inside one game. That covers its board cells, flags, counters, status and `MoveCount`.

**Why the repository lock isn't enough:** by the time the move runs, the repository lock has already been released. Two requests can each hold the same `*Game` pointer and mutate it at the same time.

**Why a move needs protecting:** `ApplyMove` is a multi-step read-modify-write. It writes one value, rewrites up to `3·size` invalid flags, updates two counters and possibly changes the status. Concurrent moves on the same game are realistic: a double click, two open tabs, or a client retrying after a timeout.

Here is what goes wrong without the lock. Requests A and B both target the empty cell r0c2.

| Step | Request A: put 5 at r0c2 | Request B: put 7 at r0c2 |
|---|---|---|
| 1 | Reads that the cell is empty, so it will increment `filledCount` | |
| 2 | | Reads that the cell is empty, so it will increment `filledCount` |
| 3 | Writes 5 and increments `filledCount` | |
| 4 | | Writes 7 and increments `filledCount` |
| 5 | Re-flags its units while B's 7 is in the cell | |

The result:

- **Inflated count.** `filledCount` went up by 2 for a single cell. The game can now be marked `COMPLETED` while one cell is still empty, or a later clear can drive the count wrong in the other direction.
- **Wrong conflicts.** A's response lists conflicts computed against B's value.
- **Torn snapshot.** A snapshot taken mid-move can show the new value with stale flags.

With the game lock, the two moves are serialized. B runs after A completes, sees the 5, overwrites it with 7, and adjusts the counters correctly, because the cell was no longer empty.

**Why one lock per game and not one global lock:**

- Games are independent. A global lock would make every user's move wait behind every other user's move.
- With a per-game lock, contention only happens between requests for the *same* game, which is rare and exactly where serialization is needed.

**How it is used:**

| Caller | Lock taken | Reason |
|---|---|---|
| `ApplyMove` | `Lock` | It mutates the board. |
| `View` / `GetGame` | `RLock` | Many readers can snapshot together, and they wait only while a move is in progress. |

- **Snapshots are taken inside the lock.** Every response then shows the board exactly as it was after that move.
- **The completion check happens inside the lock.** Two "last" moves can't both declare a win.

### 9.3 Deadlock avoidance

The two locks are **never held at the same time**. The controller always follows this order:

1. Call `games.Get`, which takes and releases the repository lock.
2. Call `game.ApplyMove`, which takes and releases the game lock.

There is no nesting, so there is no lock-ordering cycle.

### 9.4 Alternative considered

One goroutine per game, receiving moves on a channel (an actor model), would also serialize moves. It's heavier for an in-memory LLD, and it needs lifecycle management such as stopping idle goroutines. A mutex gives the same guarantee more simply.

---

## 10. Controller / API

```go
type SudokuController struct {
    users     UserRepository
    games     GameRepository
    generator PuzzleGenerator
    validator MoveValidator // StandardValidator in v1; injected into each new Game
    ids       IDGenerator   // atomic counter → "user-1", "game-7"
}

func (c *SudokuController) CreateUser(req CreateUserRequest) (UserResponse, error)
func (c *SudokuController) InitGame(req InitGameRequest) (GameResponse, error)
func (c *SudokuController) MakeMove(req MoveRequest) (GameResponse, error)
func (c *SudokuController) GetGame(userID, gameID string) (GameResponse, error)
```

| Method | REST mapping | Request |
|---|---|---|
| `CreateUser` | `POST /users` | `{name}` |
| `InitGame` | `POST /games` | `{userId, difficulty, boardSize?}`. `boardSize` defaults to 9; boxes are `√boardSize × √boardSize`. |
| `MakeMove` | `POST /games/{gameId}/moves` | `{userId, row, col, value}`. `value` is in `1..size`, or 0 to clear. |
| `GetGame` | `GET /games/{gameId}?userId=` | none |

### DTOs

```go
type GameResponse struct {
    GameID     string      `json:"gameId"`
    Difficulty Difficulty  `json:"difficulty"`
    Status     GameStatus  `json:"status"`
    Board      BoardView   `json:"board"`
    LastMove   *MoveResult `json:"lastMove,omitempty"` // only on MakeMove
}

type BoardView struct {
    Size    int      `json:"size"`
    Grid    [][]int  `json:"grid"`    // 0 = empty
    Fixed   [][]bool `json:"fixed"`
    Invalid [][]bool `json:"invalid"`
}
```

### MakeMove flow

```
Controller.MakeMove
  ├─ games.Get(gameId)                 [repo RLock, released]  → ErrGameNotFound
  ├─ game.UserID == req.UserID ?                                → ErrNotGameOwner
  └─ game.ApplyMove(pos, value)        [game Lock]
        ├─ status check                                         → ErrGameCompleted
        ├─ validator.ValidateInput                              → ErrOutOfBounds /
        │                                                         ErrInvalidValue / ErrFixedCell
        ├─ board.SetValue
        ├─ re-flag cells in validator.Units(pos)
        ├─ completion check
        └─ snapshot → GameResponse
```

### InitGame flow

```
Controller.InitGame
  ├─ users.Get(userId)                         → ErrUserNotFound
  ├─ validate difficulty, boardSize is a       → ErrInvalidDifficulty / ErrUnsupportedSize
  │  perfect square ≥ 4 (box = √boardSize)
  ├─ generator.Generate(boardSize, difficulty) → ErrUnsupportedSize / ErrInvalidBaseBoard
  ├─ game := NewGame(ids.Next(), userId, board, c.validator, ...)
  ├─ games.Save(game)                          [repo Lock]
  └─ return GameResponse
```

---

## 11. Errors

The errors are sentinels in `internal/errs`, so callers can match them with `errors.Is`.

| Error | Meaning |
|---|---|
| `ErrUserNotFound` | No user with that ID |
| `ErrGameNotFound` | No game with that ID |
| `ErrNotGameOwner` | The game belongs to another user |
| `ErrGameCompleted` | Move attempted on a finished game |
| `ErrOutOfBounds` | Row or column is outside `0..size-1` |
| `ErrInvalidValue` | Value is outside `1..size`, and isn't `0` (clear) |
| `ErrFixedCell` | Attempt to change a prefilled cell |
| `ErrInvalidDifficulty` | Unknown difficulty |
| `ErrUnsupportedSize` | The board size isn't a perfect square ≥ 4 (e.g. 6 or 8), or no base board of that size is stored |
| `ErrInvalidBaseBoard` | A seed grid, or a shuffled grid, isn't a valid solution |

A conflicting value is **not** an error. It returns normally with `LastMove.Valid = false`.

---

## 12. Extensibility

- **New size.** Seed a solved grid of that size and pass it as `boardSize`. The box size follows automatically.
- **Sudoku variants.** Add a new `MoveValidator`, for example `DiagonalValidator` with extra units. Game, Board and the controller don't change.
- **Per-cell features.** Pencil marks, hints or timestamps go on `Cell`.
- **Unique-solution puzzles.** Add a `UniqueRemovalGenerator` that implements `PuzzleGenerator`.
- **Persistence.** Swap in DB-backed repositories. When the service runs on several instances, the per-game mutex becomes a DB row lock or optimistic version check.

---

## 13. Test plan

- **Difficulty ranges.** Check `ClueRange` for sizes 4, 9 and 16, including the 27/54 boundaries landing in Medium.
- **Shuffler.**
  - Each transform on its own preserves validity.
  - The full pipeline, run with 10k seeds on 4×4, 9×9 and 16×16, always yields a valid solution.
- **Generator.** The clue count falls in the band, and the fixed flags match the non-zero cells.
- **Validator.**
  - Values 0 and `1..size` are accepted.
  - Values −1 and `size+1` are rejected.
  - Units are correct at the board's corners and edges.
- **Board and Game.**
  - A conflicting move flags the cell.
  - Overwriting or clearing that cell unflags it and its former partner.
  - Fixed cells are never flagged.
  - A full board with conflicts stays `IN_PROGRESS`, and a clean full board becomes `COMPLETED`.
- **Concurrency.** Run with `go test -race`.
  - 100 goroutines play moves on one game. Afterwards, `filledCount` and `invalidCount` must match a fresh recount of the board.
  - Many users create games in parallel.
- **Controller.** Covers the wrong owner, an unknown game, and one user running several games.

---

## 14. Implementation notes

The Go code in `sudoku/` follows this design, with these small refinements:

| Area | Design said | Code does | Why |
|---|---|---|---|
| Game fields | `Board`, `Status`, `MoveCount` exported | Unexported, reached only through `ApplyMove`, `State()` and `Status()` | The compiler now stops anyone from reading or writing them without taking the game lock. |
| `ApplyMove` return | `(MoveResult, BoardView, error)` | `(MoveResult, GameState, error)`, where `GameState` holds status, move count and board | Status and board come from the same locked snapshot, so a response can't show a board from one moment and a status from another. |
| Randomness | `*rand.Rand` injected | `randutil.Locked`, a mutex-wrapped `*rand.Rand` | `*rand.Rand` isn't goroutine-safe, and the generator is shared by every concurrent `InitGame`. |
| Conflict helper | inside `StandardValidator` | `model.ConflictingPeers(b, p, units)` | Go has no virtual dispatch through embedding, so a variant validator reuses this helper with its own units. |
| Extra API | none | `ListGames(userID)`, which maps to `GET /users/{userId}/games` | It shows the multiple games a user has running. |
| Debug check | none | `Game.Verify()` recomputes every flag and counter from scratch | Tests use it to prove the incremental bookkeeping is right, including after the 100-goroutine race test. |
| Wiring | in `main` | `internal/app.New(seed)` | Tests and the demo build the same object graph. |
| Seeds | "a few 9×9 and one 4×4" | Two hand-written 9×9 grids, plus canonical 4×4, 9×9 and 16×16 grids from `CanonicalGrid(size)` | 16×16 works out of the box. |
| Size input | — | Every public function takes the board size; `BoardView` still reports the derived `boxSize` | Clients can draw box borders without computing √size themselves. |
| Demo path | `cmd/main.go` | `cmd/sudoku/main.go` | This is the standard Go layout for a named binary. |
| HTTP server | out of scope | `cmd/server` and `internal/httpapi`. It listens on `:8080` or `$PORT`, uses Go's standard `net/http` method-and-path routing, and shuts down gracefully on Ctrl+C. | The handlers are thin adapters, so no game logic moved. |
| Move body over HTTP | `{userId,row,col,value}` | `row`, `col` and `value` are required, and a missing field returns 400 | Otherwise a forgotten `value` would decode as 0 and silently clear the cell. |
| Error → HTTP status | none | 404 not found, 403 wrong owner, 409 fixed cell or finished game, 400 bad input, 500 otherwise | A conflicting move is not an error and returns 200 with `lastMove.valid=false`. |
