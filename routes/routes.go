// Package routes provides a small router built on [http.ServeMux] that
// supports middleware and introspection of the registered routes.
package routes

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"runtime"
)

// Middleware wraps an http.Handler, returning a new http.Handler that
// typically runs some logic before and/or after calling the wrapped one.
// A middleware can short-circuit a request by not calling the wrapped handler.
type Middleware interface {
	Wrap(next http.Handler) http.Handler
}

// MiddlewareFunc adapts an ordinary function to the [Middleware] interface.
type MiddlewareFunc func(next http.Handler) http.Handler

func (f MiddlewareFunc) Wrap(next http.Handler) http.Handler {
	return f(next)
}

// Chain wraps h with the given middleware so that the first middleware
// is the outermost: a request flows through mw[0], mw[1], ..., then h.
func Chain(h http.Handler, mw ...Middleware) http.Handler {
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i].Wrap(h)
	}
	return h
}

// Router is the interface services use to register their routes.
// Paths use [http.ServeMux] pattern syntax (e.g. "/items/{id}" or
// "/files/{path...}"). An empty method matches all methods.
//
// Middleware added with Use only applies to routes registered after
// the call to Use.
type Router interface {
	Handle(method, path string, handler http.Handler)
	HandleFunc(method, path string, handler http.HandlerFunc)
	GET(path string, handler http.HandlerFunc)
	POST(path string, handler http.HandlerFunc)
	PUT(path string, handler http.HandlerFunc)
	PATCH(path string, handler http.HandlerFunc)
	DELETE(path string, handler http.HandlerFunc)
	Use(mw ...Middleware)
}

// Route describes a registered route.
type Route struct {
	Method string
	Path   string
	// HandlerName is the fully qualified name of the route's handler,
	// e.g. github.com/org/proj/foo.(*FooHandlers).Get-fm
	HandlerName string
}

type routeContextKey struct{}

// RouteFromContext returns the Route that matched the current request.
// It is available to all middleware registered on the route.
func RouteFromContext(ctx context.Context) (Route, bool) {
	rte, ok := ctx.Value(routeContextKey{}).(Route)
	return rte, ok
}

// Mux is the standard [Router] implementation and an [http.Handler].
type Mux struct {
	serveMux   *http.ServeMux
	notFound   http.Handler
	middleware []Middleware
	routes     []Route
}

var _ Router = (*Mux)(nil)

func NewMux() *Mux {
	return &Mux{
		serveMux: http.NewServeMux(),
		notFound: http.NotFoundHandler(),
	}
}

// NotFound sets the handler used when no route matches the request,
// including when the path matches but the method does not. Middleware
// added to the Mux with Use is applied to it.
func (m *Mux) NotFound(h http.Handler) {
	m.notFound = h
}

// Routes returns all routes registered on the Mux.
func (m *Mux) Routes() []Route {
	return append([]Route(nil), m.routes...)
}

func (m *Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if _, pattern := m.serveMux.Handler(r); pattern == "" {
		rte := Route{Method: r.Method, Path: r.URL.Path, HandlerName: handlerName(m.notFound)}
		withRoute(rte, Chain(m.notFound, m.middleware...)).ServeHTTP(w, r)
		return
	}
	m.serveMux.ServeHTTP(w, r)
}

func withRoute(rte Route, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), routeContextKey{}, rte)))
	})
}

func (m *Mux) Handle(method, path string, handler http.Handler) {
	rte := Route{
		Method:      method,
		Path:        path,
		HandlerName: handlerName(handler),
	}
	pattern := path
	if method != "" {
		pattern = method + " " + pattern
	}
	m.serveMux.Handle(pattern, withRoute(rte, Chain(handler, m.middleware...)))
	m.routes = append(m.routes, rte)
}

func (m *Mux) HandleFunc(method, path string, handler http.HandlerFunc) {
	m.Handle(method, path, handler)
}

func (m *Mux) GET(path string, handler http.HandlerFunc) {
	m.Handle(http.MethodGet, path, handler)
}

func (m *Mux) POST(path string, handler http.HandlerFunc) {
	m.Handle(http.MethodPost, path, handler)
}

func (m *Mux) PUT(path string, handler http.HandlerFunc) {
	m.Handle(http.MethodPut, path, handler)
}

func (m *Mux) PATCH(path string, handler http.HandlerFunc) {
	m.Handle(http.MethodPatch, path, handler)
}

func (m *Mux) DELETE(path string, handler http.HandlerFunc) {
	m.Handle(http.MethodDelete, path, handler)
}

func (m *Mux) Use(mw ...Middleware) {
	m.middleware = append(m.middleware, mw...)
}

func handlerName(h http.Handler) string {
	if f, ok := h.(http.HandlerFunc); ok {
		if fn := runtime.FuncForPC(reflect.ValueOf(f).Pointer()); fn != nil {
			return fn.Name()
		}
	}
	return fmt.Sprintf("%T", h)
}
