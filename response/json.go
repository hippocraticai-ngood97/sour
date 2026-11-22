package response

import (
	"encoding/json"
	"net/http"
)

// JSON writes v as a JSON response body with the given status code.
// If v can't be marshaled, a 500 is written instead.
func JSON(w http.ResponseWriter, code int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	w.Write(body)
}
