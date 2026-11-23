package service

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/smw1218/sour/metrics"
	"github.com/smw1218/sour/routes"
)

type testService struct {
	Template
	shutdown func() error
}

func (ts *testService) Setup(context.Context) error  { return nil }
func (ts *testService) RegisterRoutes(routes.Router) {}

func (ts *testService) Shutdown(context.Context) error {
	if ts.shutdown != nil {
		return ts.shutdown()
	}
	return nil
}

func TestMetricsNotOnServiceRouter(t *testing.T) {
	svc := &testService{Template: NewTemplate("test", 8081)}
	router := NewRouter(slog.Default(), svc, WithMetrics(metrics.Config{Registry: prometheus.NewRegistry()}))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected /metrics to be 404 on the service router, got %d", rec.Code)
	}
}
