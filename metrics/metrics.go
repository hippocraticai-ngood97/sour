// Package metrics provides Prometheus instrumentation for HTTP routes.
package metrics

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/smw1218/sour/routes"
)

// Config configures the HTTP metrics middleware. The zero value is
// usable and registers with the Prometheus default registry.
type Config struct {
	// Namespace is prefixed to metric names, e.g. "music" gives
	// music_http_requests_total.
	Namespace string
	// Registry to register metrics with and serve from. Defaults to
	// prometheus.DefaultRegisterer and prometheus.DefaultGatherer.
	Registry *prometheus.Registry
	// Buckets for the request duration histogram, in seconds.
	// Defaults to prometheus.DefBuckets.
	Buckets []float64
	// HandlerLabel returns the value of the "handler" label for a route.
	// Defaults to the short handler name used in logs, e.g. "albums.Get".
	HandlerLabel func(routes.Route) string
	// Path the metrics endpoint is served on. Defaults to "/metrics".
	Path string
	// Port of the separate metrics listener. When used with
	// service.NewRouter, 0 means the service port plus
	// service.MetricsPortOffset.
	Port int
}

// Metrics is a [routes.Middleware] that records request counts and
// durations labeled by handler, method, path, and status code. The
// method and path labels are the route's registered pattern, not the
// raw request values.
type Metrics struct {
	cfg      Config
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

var _ routes.Middleware = (*Metrics)(nil)

var labels = []string{"handler", "method", "path", "code"}

// New creates and registers the HTTP metrics. If identical metrics were
// already registered (e.g. by an earlier call to New), those are reused.
// It panics if the metrics can't be registered, like prometheus.MustRegister.
func New(cfg Config) *Metrics {
	if cfg.Buckets == nil {
		cfg.Buckets = prometheus.DefBuckets
	}
	if cfg.HandlerLabel == nil {
		cfg.HandlerLabel = routes.Route.ShortHandlerName
	}
	if cfg.Path == "" {
		cfg.Path = "/metrics"
	}
	m := &Metrics{cfg: cfg}
	m.requests = register(cfg.registerer(), prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: cfg.Namespace,
		Name:      "http_requests_total",
		Help:      "Total number of HTTP requests.",
	}, labels))
	m.duration = register(cfg.registerer(), prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: cfg.Namespace,
		Name:      "http_request_duration_seconds",
		Help:      "Duration of HTTP requests in seconds.",
		Buckets:   cfg.Buckets,
	}, labels))
	return m
}

func register[T prometheus.Collector](reg prometheus.Registerer, c T) T {
	if err := reg.Register(c); err != nil {
		var are prometheus.AlreadyRegisteredError
		if errors.As(err, &are) {
			return are.ExistingCollector.(T)
		}
		panic(err)
	}
	return c
}

func (cfg Config) registerer() prometheus.Registerer {
	if cfg.Registry != nil {
		return cfg.Registry
	}
	return prometheus.DefaultRegisterer
}

func (cfg Config) gatherer() prometheus.Gatherer {
	if cfg.Registry != nil {
		return cfg.Registry
	}
	return prometheus.DefaultGatherer
}

// Path is the configured path for the metrics endpoint.
func (m *Metrics) Path() string {
	return m.cfg.Path
}

// Port is the configured port for the metrics listener; 0 if unset.
func (m *Metrics) Port() int {
	return m.cfg.Port
}

// Handler serves the metrics for scraping.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.cfg.gatherer(), promhttp.HandlerOpts{})
}

func (m *Metrics) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)

		rte, _ := routes.RouteFromContext(r.Context())
		lv := []string{m.cfg.HandlerLabel(rte), rte.Method, rte.Path, strconv.Itoa(rec.code())}
		m.requests.WithLabelValues(lv...).Inc()
		m.duration.WithLabelValues(lv...).Observe(time.Since(start).Seconds())
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	if sr.status == 0 {
		sr.status = code
	}
	sr.ResponseWriter.WriteHeader(code)
}

func (sr *statusRecorder) Write(b []byte) (int, error) {
	if sr.status == 0 {
		sr.status = http.StatusOK
	}
	return sr.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController reach the underlying writer for
// Flush, Hijack, etc.
func (sr *statusRecorder) Unwrap() http.ResponseWriter {
	return sr.ResponseWriter
}

func (sr *statusRecorder) code() int {
	if sr.status == 0 {
		return http.StatusOK
	}
	return sr.status
}
