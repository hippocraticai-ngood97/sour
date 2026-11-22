package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

// ShutdownTimeout bounds how long Run waits for in-flight requests and
// the metrics server to finish after a shutdown signal. It's kept under
// Kubernetes' default 30s termination grace period.
var ShutdownTimeout = 25 * time.Second

// Run serves the router on the service port and, if metrics are enabled,
// serves metrics on a separate port so they aren't exposed alongside the
// service's public routes.
//
// On SIGINT/SIGTERM (or ctx being canceled), Run stops accepting requests
// and waits up to [ShutdownTimeout] for in-flight requests to finish, then
// calls the service's Shutdown, then stops the metrics server. Metrics stay
// up through the service's Shutdown so background work can still report.
// A second signal exits immediately.
func Run(ctx context.Context, si ServiceInterface, router *Router) {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	port := si.DefaultPort()
	overridePort := os.Getenv("SERVICEPORT")
	if overridePort != "" {
		parsed, err := strconv.Atoi(overridePort)
		if err == nil {
			port = parsed
		}
	}

	var metricsSrv *http.Server
	if router.metrics != nil {
		metricsPort := router.metrics.Port()
		if metricsPort == 0 {
			metricsPort = port + MetricsPortOffset
		}
		mux := http.NewServeMux()
		mux.Handle("GET "+router.metrics.Path(), router.metrics.Handler())
		metricsSrv = &http.Server{
			Addr:    fmt.Sprintf(":%d", metricsPort),
			Handler: mux,
		}
		go func() {
			slog.Info(fmt.Sprintf("Running Metrics Server on %v", metricsSrv.Addr))
			err := metricsSrv.ListenAndServe()
			if !errors.Is(err, http.ErrServerClosed) {
				slog.Error("Metrics Server failed", "error", err)
			}
		}()
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: router,
	}
	serveErr := make(chan error, 1)
	go func() {
		slog.Info(fmt.Sprintf("Running Server on %v", srv.Addr))
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		slog.Info("Got done signal, shutting down HTTP Server")
	case err := <-serveErr:
		slog.Error("Server failed", "error", err)
	}
	// restore default signal handling so a second signal kills the process
	stop()

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), ShutdownTimeout)
	defer cancel()

	err := srv.Shutdown(shutdownCtx)
	if err != nil {
		slog.Error("Server Shutdown Failed", "error", err)
	}

	slog.Info("Calling Service Shutdown")
	err = si.Shutdown()
	if err != nil {
		slog.Error("Failed service Shutdown", "error", err)
	}

	if metricsSrv != nil {
		err = metricsSrv.Shutdown(shutdownCtx)
		if err != nil {
			slog.Error("Metrics Server Shutdown Failed", "error", err)
		}
	}
}
