package apimodels

import (
	"context"

	"github.com/llimllib/hatchat/server/db"
	"github.com/llimllib/hatchat/server/models"
)

type Room struct {
	ID string
}

func UserRooms(ctx context.Context, db *db.DB, uid string) ([]*Room, error) {
	rooms, err := models.RoomsByUserID(context.Background(), db, uid)
	if err != nil {
		return nil, err
	}

	apirooms := []*Room{}
	for _, room := range rooms {
		apirooms = append(apirooms, &Room{room.RoomID})
	}

	return apirooms, nil
}
