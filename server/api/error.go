package api

type ErrorBody struct {
	Message string
}

func must[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}

// ErrorMessage returns a message struct to be sent to the client
func (a *Api) ErrorMessage(message string) *Envelope {
	return &Envelope{
		Type: "error",
		Data: &ErrorBody{message},
	}
}
