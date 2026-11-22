package service

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/smw1218/sour/logger"
	"github.com/smw1218/sour/response"
	"github.com/smw1218/sour/routes"
)

// NewRouter creates the root router for a service with the standard
// version and health endpoints, logging, and panic recovery.
func NewRouter(log *slog.Logger, svc ServiceInterface) *routes.Mux {
	m := routes.NewMux()
	m.NotFound(http.HandlerFunc(NotFoundHandler))
	m.GET(fmt.Sprintf("/%v/version", svc.LongName()), BuildInfo)
	m.GET(fmt.Sprintf("/%v/health", svc.LongName()), HealthHandler)
	m.Use(
		logger.NewLoggerMiddleware(log),
		Recovery())
	// TODO add monitoring etc

	return m
}

func NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	response.NotFound(w, "")
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"up"}`))
}

// Recovery logs panics from downstream handlers and responds with a 500.
func Recovery() routes.Middleware {
	return routes.MiddlewareFunc(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				logger.Get(r.Context()).Error("Recovered from panic", "panic", rec, "stack", string(debug.Stack()))
				response.NewError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError)).Respond(w)
			}()
			next.ServeHTTP(w, r)
		})
	})
}
