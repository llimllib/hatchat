package api

import (
	"log/slog"

	"github.com/llimllib/hatchat/server/db"
)

type Api struct {
	db     *db.DB
	logger *slog.Logger
}

func NewApi(db *db.DB, logger *slog.Logger) *Api {
	return &Api{db, logger}
}

type Envelope struct {
	Type string
	Data any
}
