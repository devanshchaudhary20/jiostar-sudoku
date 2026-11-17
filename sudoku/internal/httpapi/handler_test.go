package httpapi_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"jiostar/sudoku/internal/app"
	"jiostar/sudoku/internal/controller"
	"jiostar/sudoku/internal/httpapi"
)

func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	ctrl, err := app.New(5)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(httpapi.NewHandler(ctrl, nil))
	t.Cleanup(srv.Close)
	return srv
}

// call sends a request and decodes the JSON response into out (if non-nil).
func call(t *testing.T, method, url, body string, out any) int {
	t.Helper()
	req, err := http.NewRequest(method, url, bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("%s %s: content-type %q", method, url, ct)
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("%s %s: decode: %v", method, url, err)
		}
	}
	return resp.StatusCode
}

func TestFullFlowOverHTTP(t *testing.T) {
	srv := newServer(t)

	var user controller.UserResponse
	if code := call(t, "POST", srv.URL+"/users", `{"name":"devansh"}`, &user); code != http.StatusCreated {
		t.Fatalf("create user: %d", code)
	}

	var game controller.GameResponse
	body := fmt.Sprintf(`{"userId":%q,"difficulty":"easy","boardSize":4}`, user.UserID)
	if code := call(t, "POST", srv.URL+"/games", body, &game); code != http.StatusCreated {
		t.Fatalf("init game: %d", code)
	}
	if game.Board.Size != 4 || game.Board.BoxSize != 2 || game.Status != "IN_PROGRESS" {
		t.Fatalf("unexpected game %+v", game)
	}

	// Find an empty cell and a value already in its row → conflicting move.
	r, c, clash := -1, -1, 0
	for i, row := range game.Board.Grid {
		for j, v := range row {
			if v == 0 && r < 0 {
				r, c = i, j
			}
		}
	}
	for j, v := range game.Board.Grid[r] {
		if game.Board.Fixed[r][j] {
			clash = v
		}
	}
	if clash == 0 { // row has no given; fall back to its column
		for i := range game.Board.Grid {
			if game.Board.Fixed[i][c] {
				clash = game.Board.Grid[i][c]
			}
		}
	}

	var moved controller.GameResponse
	moveURL := srv.URL + "/games/" + game.GameID + "/moves"
	body = fmt.Sprintf(`{"userId":%q,"row":%d,"col":%d,"value":%d}`, user.UserID, r, c, clash)
	if code := call(t, "POST", moveURL, body, &moved); code != http.StatusOK {
		t.Fatalf("move: %d", code)
	}
	if moved.LastMove == nil || moved.LastMove.Valid || !moved.Board.Invalid[r][c] {
		t.Fatalf("conflicting move should be placed and flagged: %+v", moved.LastMove)
	}

	var got controller.GameResponse
	if code := call(t, "GET", srv.URL+"/games/"+game.GameID+"?userId="+user.UserID, "", &got); code != http.StatusOK {
		t.Fatalf("get game: %d", code)
	}
	if got.Board.Grid[r][c] != clash || got.MoveCount != 1 {
		t.Fatal("GET should return the board after the move")
	}

	var list []controller.GameSummary
	if code := call(t, "GET", srv.URL+"/users/"+user.UserID+"/games", "", &list); code != http.StatusOK || len(list) != 1 {
		t.Fatalf("list games: %d %v", code, list)
	}

	if code := call(t, "GET", srv.URL+"/healthz", "", nil); code != http.StatusOK {
		t.Fatalf("healthz: %d", code)
	}
}

func TestErrorStatusCodes(t *testing.T) {
	srv := newServer(t)
	var user controller.UserResponse
	call(t, "POST", srv.URL+"/users", `{"name":"a"}`, &user)
	var other controller.UserResponse
	call(t, "POST", srv.URL+"/users", `{"name":"b"}`, &other)
	var game controller.GameResponse
	call(t, "POST", srv.URL+"/games", fmt.Sprintf(`{"userId":%q,"difficulty":"medium"}`, user.UserID), &game)

	fixedR, fixedC := -1, -1
	for i, row := range game.Board.Fixed {
		for j, f := range row {
			if f && fixedR < 0 {
				fixedR, fixedC = i, j
			}
		}
	}
	moves := srv.URL + "/games/" + game.GameID + "/moves"
	u := user.UserID

	cases := []struct {
		name, method, url, body string
		want                    int
	}{
		{"empty name", "POST", srv.URL + "/users", `{"name":""}`, 400},
		{"malformed json", "POST", srv.URL + "/users", `{"name":`, 400},
		{"unknown field", "POST", srv.URL + "/users", `{"name":"x","age":3}`, 400},
		{"unknown user", "POST", srv.URL + "/games", `{"userId":"nope","difficulty":"easy"}`, 404},
		{"bad difficulty", "POST", srv.URL + "/games", fmt.Sprintf(`{"userId":%q,"difficulty":"insane"}`, u), 400},
		{"board size 6", "POST", srv.URL + "/games", fmt.Sprintf(`{"userId":%q,"difficulty":"easy","boardSize":6}`, u), 400},
		{"unknown game", "POST", srv.URL + "/games/game-404/moves", fmt.Sprintf(`{"userId":%q,"row":0,"col":0,"value":1}`, u), 404},
		{"wrong owner", "POST", moves, fmt.Sprintf(`{"userId":%q,"row":0,"col":0,"value":1}`, other.UserID), 403},
		{"missing value", "POST", moves, fmt.Sprintf(`{"userId":%q,"row":0,"col":0}`, u), 400},
		{"value 10 on 9x9", "POST", moves, fmt.Sprintf(`{"userId":%q,"row":0,"col":0,"value":10}`, u), 400},
		{"out of bounds", "POST", moves, fmt.Sprintf(`{"userId":%q,"row":9,"col":0,"value":1}`, u), 400},
		{"fixed cell", "POST", moves, fmt.Sprintf(`{"userId":%q,"row":%d,"col":%d,"value":1}`, u, fixedR, fixedC), 409},
		{"get without userId", "GET", srv.URL + "/games/" + game.GameID, "", 400},
		{"wrong method", "DELETE", srv.URL + "/games/" + game.GameID, "", 405},
	}
	for _, tc := range cases {
		var e struct {
			Error string `json:"error"`
		}
		var out any = &e
		if tc.want == 405 {
			out = nil // the mux's own 405 response isn't JSON
			req, _ := http.NewRequest(tc.method, tc.url, nil)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != tc.want {
				t.Errorf("%s: got %d, want %d", tc.name, resp.StatusCode, tc.want)
			}
			continue
		}
		if code := call(t, tc.method, tc.url, tc.body, out); code != tc.want {
			t.Errorf("%s: got %d (%s), want %d", tc.name, code, e.Error, tc.want)
		} else if e.Error == "" {
			t.Errorf("%s: error body missing", tc.name)
		}
	}
}
