package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/smw1218/sour/metrics"
)

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func portAvailable(port int) bool {
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	l.Close()
	return true
}

// testClient doesn't pool connections; a pooled client can leave an unused
// connection open, which makes http.Server.Shutdown wait 5s before closing it.
var testClient = &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}

func waitForServer(t *testing.T, url string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := testClient.Get(url)
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server at %v never came up", url)
}

func get(url string) (int, string, error) {
	resp, err := testClient.Get(url)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body), err
}

func TestRunGracefulShutdownOrder(t *testing.T) {
	port := freePort(t)
	metricsPort := freePort(t)
	t.Setenv("SERVICEPORT", strconv.Itoa(port))
	metricsURL := fmt.Sprintf("http://localhost:%d/metrics", metricsPort)

	var metricsStatusDuringShutdown int
	svc := &testService{
		Template: NewTemplate("test", 0),
		shutdown: func() error {
			code, _, err := get(metricsURL)
			metricsStatusDuringShutdown = code
			return err
		},
	}
	router := NewRouter(slog.Default(), svc, WithMetrics(metrics.Config{
		Registry: prometheus.NewRegistry(),
		Port:     metricsPort,
	}))
	router.HandleFunc("GET /slow", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.Write([]byte("done"))
	})

	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan struct{})
	go func() {
		Run(ctx, svc, router)
		close(runDone)
	}()
	waitForServer(t, fmt.Sprintf("http://localhost:%d/test-service/health", port))

	type result struct {
		code int
		body string
		err  error
	}
	slowResult := make(chan result, 1)
	go func() {
		code, body, err := get(fmt.Sprintf("http://localhost:%d/slow", port))
		slowResult <- result{code, body, err}
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()

	res := <-slowResult
	if res.err != nil || res.code != http.StatusOK || res.body != "done" {
		t.Fatalf("in-flight request didn't complete: %d %q %v", res.code, res.body, res.err)
	}
	select {
	case <-runDone:
	case <-time.After(5 * time.Second):
		t.Fatal("Run didn't return after shutdown")
	}

	if metricsStatusDuringShutdown != http.StatusOK {
		t.Fatalf("expected metrics to be up during service Shutdown, got %d", metricsStatusDuringShutdown)
	}
	if !portAvailable(metricsPort) {
		t.Fatal("expected metrics server to be stopped after Run returned")
	}
}

func TestRunMetricsDefaultPortOffset(t *testing.T) {
	port := freePort(t)
	if !portAvailable(port + MetricsPortOffset) {
		t.Skipf("port %d is in use", port+MetricsPortOffset)
	}
	t.Setenv("SERVICEPORT", strconv.Itoa(port))
	svc := &testService{Template: NewTemplate("test", 0)}
	router := NewRouter(slog.Default(), svc, WithMetrics(metrics.Config{Registry: prometheus.NewRegistry()}))

	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan struct{})
	go func() {
		Run(ctx, svc, router)
		close(runDone)
	}()
	defer func() {
		cancel()
		<-runDone
	}()

	metricsURL := fmt.Sprintf("http://localhost:%d/metrics", port+MetricsPortOffset)
	waitForServer(t, metricsURL)
	code, _, err := get(metricsURL)
	if err != nil || code != http.StatusOK {
		t.Fatalf("expected metrics on service port + %d, got %d %v", MetricsPortOffset, code, err)
	}
}
