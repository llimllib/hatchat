package models

import (
	"context"
	"fmt"
	"time"

	"github.com/llimllib/tinychat/server/db"
)

type User struct {
	ID         string
	Username   string
	Password   string
	CreatedAt  time.Time
	ModifiedAt time.Time
}

func (u *User) Insert(db *db.DB) error {
	_, err := db.ExecContext(context.Background(), `
		INSERT INTO users (id, username, password, created_at, modified_at)
		VALUES (?, ?, ?, ?, ?)
	`, u.ID, u.Username, u.Password, time.Now(), time.Now())
	if err != nil {
		return err
	}

	return nil
}

func GetUserByUsername(db *db.DB, username string) (*User, error) {
	var user User
	rows, err := db.QueryContext(context.Background(), `
		SELECT id, username, password, created_at, modified_at
		FROM users
		WHERE username = ?
	`, username)
	if !rows.Next() || err != nil {
		return nil, fmt.Errorf("user not found: %s", username)
	}
	defer rows.Close()

	err = rows.Scan(&user.ID, &user.Username, &user.Password, &user.CreatedAt, &user.ModifiedAt)
	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}

	return &user, nil
}

func GetUserByID(db *db.DB, id string) (*User, error) {
	var user User
	rows, err := db.QueryContext(context.Background(), `
		SELECT id, username, password, created_at, modified_at
		FROM users
		WHERE id = ?
	`, id)
	if !rows.Next() || err != nil {
		return nil, fmt.Errorf("user not found: %s", id)
	}
	defer rows.Close()

	err = rows.Scan(&user.ID, &user.Username, &user.Password, &user.CreatedAt, &user.ModifiedAt)
	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}

	return &user, nil
}
