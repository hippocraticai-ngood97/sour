package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func tagMiddleware(tag string) Middleware {
	return MiddlewareFunc(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(tag + ">"))
			next.ServeHTTP(w, r)
		})
	})
}

func writeBody(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}
}

func serve(m *Mux, method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func TestMiddlewareOrder(t *testing.T) {
	m := NewMux()
	m.HandleFunc("GET /before", writeBody("before"))
	m.Use(tagMiddleware("a"), tagMiddleware("b"))
	m.HandleFunc("GET /items/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("item " + r.PathValue("id")))
	})
	m.HandleFunc("GET /special", writeBody("special"), tagMiddleware("c"), tagMiddleware("d"))
	m.HandleFunc("GET /plain", writeBody("plain"))

	cases := map[string]string{
		"/before":  "before",
		"/items/7": "a>b>item 7",
		"/special": "a>b>c>d>special",
		"/plain":   "a>b>plain",
	}
	for path, expected := range cases {
		rec := serve(m, http.MethodGet, path)
		if rec.Code != http.StatusOK || rec.Body.String() != expected {
			t.Errorf("GET %v: expected 200 %q, got %d %q", path, expected, rec.Code, rec.Body.String())
		}
	}
}

func TestNotFound(t *testing.T) {
	m := NewMux()
	m.Use(MiddlewareFunc(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Middleware", "yes")
			next.ServeHTTP(w, r)
		})
	}))
	m.NotFound(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("custom"))
	}))
	m.HandleFunc("GET /exists", writeBody("ok"))

	for _, req := range [][2]string{{http.MethodGet, "/missing"}, {http.MethodPost, "/exists"}} {
		rec := serve(m, req[0], req[1])
		if rec.Code != http.StatusNotFound || rec.Body.String() != "custom" || rec.Header().Get("X-Middleware") != "yes" {
			t.Errorf("%v %v: expected custom 404 with middleware, got %d %q %v", req[0], req[1], rec.Code, rec.Body.String(), rec.Header())
		}
	}
}

func TestRoutesAndRouteContext(t *testing.T) {
	m := NewMux()
	var seen Route
	m.Use(MiddlewareFunc(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seen, _ = RouteFromContext(r.Context())
			next.ServeHTTP(w, r)
		})
	}))
	m.HandleFunc("POST /v1/things", namedHandler)

	routes := m.Routes()
	if len(routes) != 1 || routes[0].Pattern != "POST /v1/things" {
		t.Fatalf("unexpected routes %+v", routes)
	}
	serve(m, http.MethodPost, "/v1/things")
	if seen.Pattern != "POST /v1/things" || !strings.HasSuffix(seen.HandlerName, "routes.namedHandler") {
		t.Fatalf("unexpected route in context %+v", seen)
	}
}

func namedHandler(w http.ResponseWriter, r *http.Request) {}

func TestShortHandlerName(t *testing.T) {
	cases := map[string]string{
		"github.com/smw1218/sour/cmd/test-service/te.(*TEHandlers).Test-fm":                        "te.Test",
		"github.com/smw1218/sour/cmd/test-service/te.TopLevel":                                     "te.TopLevel",
		"github.com/smw1218/sour/cmd/test-service/app.(*TestService).RegisterRoutes.Wrapped.func1": "app.RegisterRoutes.Wrapped.func1",
	}
	for input, expected := range cases {
		actual := Route{HandlerName: input}.ShortHandlerName()
		if actual != expected {
			t.Fatalf("Failed %v, expected %v got %v", input, expected, actual)
		}
	}
}
