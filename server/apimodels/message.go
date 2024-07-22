package apimodels

type Message struct {
	ID         string `json:"id"`
	RoomID     string `json:"room_id"`
	UserID     string `json:"user_id"`
	Body       string `json:"body"`
	CreatedAt  string `json:"created_at"`
	ModifiedAt string `json:"modified_at"`
}

func NewMessage(ID, roomID, userID, body, createdAt, modifiedAt string) *Message {
	return &Message{
		ID:         ID,
		RoomID:     roomID,
		UserID:     userID,
		Body:       body,
		CreatedAt:  createdAt,
		ModifiedAt: modifiedAt,
	}
}
