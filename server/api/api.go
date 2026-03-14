package api

import (
	"log/slog"

	"github.com/llimllib/hatchat/server/db"
	"github.com/llimllib/hatchat/server/protocol"
)

// OnlineUsersProvider is an interface for querying which users are currently online.
// This allows the API layer to check presence without depending on the hub directly.
type OnlineUsersProvider interface {
	OnlineUserIDs() []string
}

type Api struct {
	db             *db.DB
	logger         *slog.Logger
	onlineProvider OnlineUsersProvider
}

func NewApi(db *db.DB, logger *slog.Logger, onlineProvider OnlineUsersProvider) *Api {
	return &Api{db, logger, onlineProvider}
}

// Envelope is an alias for protocol.Envelope for convenience within this package
type Envelope = protocol.Envelope
