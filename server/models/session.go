package models

import (
	"context"
	"fmt"
	"time"

	"github.com/llimllib/tinychat/server/db"
)

type Session struct {
	ID        string
	Username  string
	CreatedAt time.Time
}

func (u *Session) Insert(db *db.DB) error {
	_, err := db.Exec(context.Background(), `
		INSERT INTO sessions (id, username, created_at)
		VALUES (?, ?, ?)
	`, u.ID, u.Username, time.Now())
	if err != nil {
		return err
	}

	return nil
}

func GetSessionByUsername(db *db.DB, username string) (*Session, error) {
	var session Session
	rows, err := db.Select(context.Background(), `
		SELECT id, username, created_at
		FROM sessions 
		WHERE username = ?
	`, username)
	if !rows.Next() || err != nil {
		return nil, fmt.Errorf("session not found: %s", username)
	}
	defer rows.Close()

	err = rows.Scan(&session.ID, &session.Username, &session.CreatedAt)
	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}

	return &session, nil
}

func GetSessionByID(db *db.DB, id string) (*Session, error) {
	var session Session
	rows, err := db.Select(context.Background(), `
		SELECT id, username, created_at
		FROM sessions 
		WHERE id = ?
	`, id)
	if !rows.Next() || err != nil {
		return nil, fmt.Errorf("session not found: %s", id)
	}
	defer rows.Close()

	err = rows.Scan(&session.ID, &session.Username, &session.CreatedAt)
	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}

	return &session, nil
}
