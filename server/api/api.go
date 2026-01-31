package api

import (
	"log/slog"

	"github.com/llimllib/hatchat/server/db"
	"github.com/llimllib/hatchat/server/protocol"
)

type Api struct {
	db     *db.DB
	logger *slog.Logger
}

func NewApi(db *db.DB, logger *slog.Logger) *Api {
	return &Api{db, logger}
}

// Envelope is an alias for protocol.Envelope for convenience within this package
type Envelope = protocol.Envelope
