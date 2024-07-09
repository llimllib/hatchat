package models

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// generateSessionID generates a random session ID
func GenerateSessionID() string {
	b := make([]byte, 32)
	rand.Read(b) //nolint: errcheck
	return base64.URLEncoding.EncodeToString(b)
}

// generateRoomID generates a room ID
func GenerateRoomID() string {
	b := make([]byte, 6)
	rand.Read(b) //nolint: errcheck
	return fmt.Sprintf("roo_%s", hex.EncodeToString(b))
}

// generateRoomID generates a message ID
func GenerateMessageID() string {
	b := make([]byte, 6)
	rand.Read(b) //nolint: errcheck
	return fmt.Sprintf("msg_%s", hex.EncodeToString(b))
}

func GenerateUserID() string {
	b := make([]byte, 8)
	rand.Read(b) //nolint: errcheck
	return fmt.Sprintf("usr_%s", hex.EncodeToString(b))
}
