package api

import "encoding/json"

type ErrorBody struct {
	Message string
}

func must[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}

// ErrorMessage returns a JSON-formatted error message to be sent to the client
func (a *Api) ErrorMessage(message string) []byte {
	env := &Envelope{
		Type: "error",
		Data: &ErrorBody{message},
	}

	return must(json.Marshal(env))
}
