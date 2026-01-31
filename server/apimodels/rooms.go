package apimodels

import (
	"context"

	"github.com/llimllib/hatchat/server/db"
	"github.com/llimllib/hatchat/server/models"
)

type Room struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsPrivate bool   `json:"is_private"`
}

func UserRooms(ctx context.Context, db *db.DB, uid string) ([]*Room, error) {
	rooms, err := models.UserRoomDetailsByUserID(ctx, db, uid)
	if err != nil {
		return nil, err
	}

	apirooms := []*Room{}
	for _, room := range rooms {
		apirooms = append(apirooms, &Room{
			ID:        room.ID,
			Name:      room.Name,
			IsPrivate: room.IsPrivate != 0,
		})
	}

	return apirooms, nil
}
