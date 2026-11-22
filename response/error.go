package response

import (
	"net/http"
)

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func NewError(code int, message string) Error {
	return Error{
		Code:    code,
		Message: message,
	}
}

func (err Error) Respond(w http.ResponseWriter) {
	JSON(w, err.Code, err)
}
