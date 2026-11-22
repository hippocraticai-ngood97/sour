package logger

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"strings"
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
				handlerLogger = log.With("hand", HandlerName(rte.HandlerName))
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

// HandlerName makes the handler names shorter and readable ala:
// github.com/smw1218/sour/cmd/test-service/te.(*TEHandlers).Test-fm -> te.Test
// github.com/smw1218/sour/cmd/test-service/te.TopLevel -> te.TopLevel
func HandlerName(fullType string) string {
	slashed := strings.Split(fullType, "/")
	last := slashed[len(slashed)-1]
	last = strings.TrimSuffix(last, "-fm")
	dotted := strings.Split(last, ".")
	if len(dotted) <= 2 {
		return last
	}
	return fmt.Sprintf("%s.%s", dotted[0], strings.Join(dotted[2:], "."))
}
