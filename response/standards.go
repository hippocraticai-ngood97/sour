package response

import (
	"fmt"
	"net/http"

	"github.com/smw1218/sour/env"
	"github.com/smw1218/sour/logger"
)

// InternalServerError responses should always include an error cause.
// These are always logged when calling this method. In non-production
// environments, the err.Error is set as the message in the response for
// easier debugging. The http.StatusText(http.StatusInternalServerError)
// is returned instead in production.
func InternalServerError(w http.ResponseWriter, r *http.Request, err error) {
	// always log these
	logger.Get(r.Context()).Error("InternalServerError", "error", err)
	message := err.Error()
	// Don't respond internal issues in responses in production
	if env.Get().IsProd() {
		message = http.StatusText(http.StatusInternalServerError)
	}
	NewError(http.StatusInternalServerError, message).Respond(w)
}

// BadRequest responses should always include a message telling the
// caller why the request couldn't be processed
func BadRequest(w http.ResponseWriter, message string) {
	NewError(http.StatusBadRequest, message).Respond(w)
}

// BadRequestf responses should always include a message telling the
// caller why the request couldn't be processed
func BadRequestf(w http.ResponseWriter, format string, args ...any) {
	NewError(http.StatusBadRequest, fmt.Sprintf(format, args...)).Respond(w)
}

// NotFound generic 404, if message is empty, http.StatusText(http.StatusNotFound) is used
func NotFound(w http.ResponseWriter, message string) {
	if message == "" {
		message = http.StatusText(http.StatusNotFound)
	}
	NewError(http.StatusNotFound, message).Respond(w)
}

// NotFound 404 with a custom message, maybe saying which record wasn't found or why
func NotFoundf(w http.ResponseWriter, format string, args ...any) {
	NewError(http.StatusNotFound, fmt.Sprintf(format, args...)).Respond(w)
}
