package api

type ErrorBody struct {
	Message string
}

// ErrorMessage returns a message struct to be sent to the client
func (a *Api) ErrorMessage(message string) *Envelope {
	return &Envelope{
		Type: "error",
		Data: &ErrorBody{message},
	}
}
