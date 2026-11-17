package model

// User is a player. The user → games link lives in GameRepository,
// which indexes games by user ID.
type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
