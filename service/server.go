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
)

// Run serves the router on the service port and, if metrics are enabled,
// serves metrics on a separate port so they aren't exposed alongside the
// service's public routes.
func Run(ctx context.Context, si ServiceInterface, router *Router) {
	ctx, _ = signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	//defer stop() // I don't think stop is needed in this context

	port := si.DefaultPort()
	overridePort := os.Getenv("SERVICEPORT")
	if overridePort != "" {
		parsed, err := strconv.Atoi(overridePort)
		if err == nil {
			port = parsed
		}
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: router,
	}

	metricsSrv := router.metricsServer(port)
	if metricsSrv != nil {
		go func() {
			slog.Info(fmt.Sprintf("Running Metrics Server on %v", metricsSrv.Addr))
			err := metricsSrv.ListenAndServe()
			if !errors.Is(err, http.ErrServerClosed) {
				slog.Error("Metrics Server failed", "error", err)
			}
		}()
	}

	go func() {
		<-ctx.Done()
		slog.Info("Got done signal, shutting down HTTP Server")
		err := srv.Shutdown(ctx)
		if err != nil {
			slog.Error("Server Shutdown Failed", "error", err)
		}
		if metricsSrv != nil {
			err = metricsSrv.Shutdown(ctx)
			if err != nil {
				slog.Error("Metrics Server Shutdown Failed", "error", err)
			}
		}
	}()

	// This call blocks until shutdown. After shutdown, we know we have finished processing
	// all open hanlders
	slog.Info(fmt.Sprintf("Running Server on %v", srv.Addr))
	err := srv.ListenAndServe()
	slog.Info("ListenAndServe exiting", "error", err)

	// Once the handlers have done their work, if there's any other waiting for work to complete,
	// it can happen in the service's Shutdown handler
	slog.Info("Calling Service Shutdown")
	err = si.Shutdown()
	if err != nil {
		slog.Error("Failed service Shutdown", "error", err)
	}
}
