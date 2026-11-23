package service

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/smw1218/sour/logger"
	"github.com/smw1218/sour/metrics"
	"github.com/smw1218/sour/response"
	"github.com/smw1218/sour/routes"
)

type routerConfig struct {
	metrics *metrics.Config
}

type RouterOption func(*routerConfig)

// WithMetrics configures the Prometheus HTTP metrics middleware.
func WithMetrics(cfg metrics.Config) RouterOption {
	return func(rc *routerConfig) {
		rc.metrics = &cfg
	}
}

// WithoutMetrics disables the Prometheus HTTP metrics middleware and endpoint.
func WithoutMetrics() RouterOption {
	return func(rc *routerConfig) {
		rc.metrics = nil
	}
}

// MetricsPortOffset is added to the service port to get the metrics
// listener port when [metrics.Config.Port] is unset.
const MetricsPortOffset = 100

// Router is the root router for a service. Pass it to [Run], which serves
// the routes on the service port and metrics on a separate port.
type Router struct {
	*routes.Mux
	metrics *metrics.Metrics
}

// NewRouter creates the root router for a service with the standard
// version and health endpoints, logging, Prometheus metrics, and panic
// recovery. Metrics are enabled by default with a zero [metrics.Config];
// use [WithMetrics] or [WithoutMetrics] to change that.
func NewRouter(log *slog.Logger, svc ServiceInterface, opts ...RouterOption) *Router {
	rc := &routerConfig{metrics: &metrics.Config{}}
	for _, opt := range opts {
		opt(rc)
	}

	r := &Router{Mux: routes.NewMux()}
	r.NotFound(http.HandlerFunc(NotFoundHandler))
	r.HandleFunc(fmt.Sprintf("GET /%v/version", svc.LongName()), BuildInfo)
	r.HandleFunc(fmt.Sprintf("GET /%v/health", svc.LongName()), HealthHandler)
	middleware := []routes.Middleware{logger.NewLoggerMiddleware(log)}
	if rc.metrics != nil {
		r.metrics = metrics.New(*rc.metrics)
		middleware = append(middleware, r.metrics)
	}
	middleware = append(middleware, Recovery())
	r.Use(middleware...)

	return r
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
