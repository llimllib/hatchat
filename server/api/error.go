package api

import "github.com/llimllib/hatchat/server/protocol"

// ErrorMessage returns a message struct to be sent to the client
func (a *Api) ErrorMessage(message string) *Envelope {
	return ErrorResponse(message)
}

// ErrorResponse returns an error envelope without needing an Api instance
func ErrorResponse(message string) *Envelope {
	return &Envelope{
		Type: "error",
		Data: &protocol.ErrorResponse{Message: message},
	}
}
