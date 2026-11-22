package service

import (
	"net/http"

	"github.com/smw1218/sour/response"
)

var (
	Version   string
	Builder   string
	BuildTime string
)

func init() {
	if Version == "" {
		Version = "development"
	}
}

func BuildInfo(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, http.StatusOK, map[string]string{
		"version":   Version,
		"builder":   Builder,
		"buildTime": BuildTime,
	})
}
