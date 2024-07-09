package apimodels

import (
	"database/sql"
)

type User struct {
	ID       string         `json:"id"`
	Username string         `json:"username"`
	Avatar   sql.NullString `json:"avatar"`
}

func NewUser(id, username string, avatar sql.NullString) *User {
	return &User{
		ID:       id,
		Username: username,
		Avatar:   avatar,
	}
}
