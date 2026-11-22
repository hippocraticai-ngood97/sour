package logger

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/lmittmann/tint"
	"github.com/smw1218/sour/env"
	"github.com/smw1218/sour/routes"
)

func SetupDefaultSlog(serviceName string) *slog.Logger {
	defaultLevel := slog.LevelDebug
	if env.Get().IsProd() {
		defaultLevel = slog.LevelInfo
	}
	slogger := slog.New(
		tint.NewHandler(log.Default().Writer(),
			&tint.Options{
				Level:      defaultLevel,
				TimeFormat: "2006-01-02 15:04:05.000",
			}))
	slogger = slogger.With("svc", serviceName)
	return slogger
}

type contextLogger string

var contextLoggerKey contextLogger = "logger"

func Get(ctx context.Context) *slog.Logger {
	loggerAny := ctx.Value(contextLoggerKey)
	if loggerAny == nil {
		return slog.Default()
	}
	return loggerAny.(*slog.Logger)
}

func SetContextLogger(ctx context.Context, newLogger *slog.Logger) context.Context {
	return context.WithValue(ctx, contextLoggerKey, newLogger)
}

func NewLoggerMiddleware(log *slog.Logger) routes.Middleware {
	return routes.MiddlewareFunc(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerLogger := log
			if rte, ok := routes.RouteFromContext(r.Context()); ok {
				handlerLogger = log.With("hand", rte.ShortHandlerName())
			}
			r = r.WithContext(SetContextLogger(r.Context(), handlerLogger))
			start := time.Now()
			next.ServeHTTP(w, r)
			latency := time.Since(start)
			endpoint := fmt.Sprintf("%v %v", r.Method, r.URL)
			handlerLogger.Info(endpoint, "tm", latency)
		})
	})
}
