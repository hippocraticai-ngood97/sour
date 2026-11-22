package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/smw1218/sour/routes"
)

type itemHandlers struct{}

func (ih *itemHandlers) Get(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusTeapot)
}

func TestMiddlewareLabels(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := New(Config{Registry: reg})
	mux := routes.NewMux()
	mux.Use(m)
	ih := &itemHandlers{}
	mux.GET("/items/{id}", ih.Get)

	for _, path := range []string{"/items/1", "/items/2", "/nope"} {
		mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	}

	expected := `
# HELP http_requests_total Total number of HTTP requests.
# TYPE http_requests_total counter
http_requests_total{code="404",handler="http.NotFound",method="",path=""} 1
http_requests_total{code="418",handler="metrics.Get",method="GET",path="/items/{id}"} 2
`
	if err := testutil.GatherAndCompare(reg, strings.NewReader(expected), "http_requests_total"); err != nil {
		t.Fatal(err)
	}
	if n := testutil.CollectAndCount(m.duration); n != 2 {
		t.Fatalf("expected 2 duration series, got %d", n)
	}
}

func TestNewReusesRegisteredMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	first := New(Config{Registry: reg})
	second := New(Config{Registry: reg})
	if first.requests != second.requests || first.duration != second.duration {
		t.Fatal("expected second New to reuse registered collectors")
	}
}
