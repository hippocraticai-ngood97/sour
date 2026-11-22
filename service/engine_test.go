package service

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/smw1218/sour/metrics"
	"github.com/smw1218/sour/routes"
)

type testService struct {
	Template
}

func (ts *testService) Setup() error                 { return nil }
func (ts *testService) RegisterRoutes(routes.Router) {}

func TestMetricsServedOnSeparatePort(t *testing.T) {
	svc := &testService{NewTemplate("test", 8081)}
	router := NewRouter(slog.Default(), svc, WithMetrics(metrics.Config{Registry: prometheus.NewRegistry()}))
	router.GET("/hello", func(w http.ResponseWriter, r *http.Request) {})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected /metrics to be 404 on the service router, got %d", rec.Code)
	}
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/hello", nil))

	metricsSrv := router.metricsServer(8081)
	if metricsSrv.Addr != ":8181" {
		t.Fatalf("expected metrics on :8181, got %v", metricsSrv.Addr)
	}
	rec = httptest.NewRecorder()
	metricsSrv.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `path="/hello"`) {
		t.Fatalf("expected metrics for /hello, got %d %q", rec.Code, rec.Body.String())
	}
}

func TestMetricsServerConfig(t *testing.T) {
	svc := &testService{NewTemplate("test", 8081)}
	router := NewRouter(slog.Default(), svc, WithMetrics(metrics.Config{Registry: prometheus.NewRegistry(), Port: 9100}))
	if addr := router.metricsServer(8081).Addr; addr != ":9100" {
		t.Fatalf("expected configured port :9100, got %v", addr)
	}

	router = NewRouter(slog.Default(), svc, WithoutMetrics())
	if router.metricsServer(8081) != nil {
		t.Fatal("expected no metrics server when metrics are disabled")
	}
}
