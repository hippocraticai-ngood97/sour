package proxy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/smw1218/sour/project"
	"github.com/smw1218/sour/routes"
	"github.com/smw1218/sour/service"
)

// ServiceProxy is a reverse proxy that routes each service's routes to
// that service. It is an http.Handler, or can listen on its own with Run.
type ServiceProxy struct {
	// Port that Run listens on. Defaults to project.DefaultStartingPort.
	Port int
	// Mux holds the proxied routes and can be further configured. The
	// proxied routes are already registered, so middleware added with
	// Mux.Use only applies to routes added afterward.
	Mux *routes.Mux
}

// NewProxy takes the full list of services and will create a reverse proxy
// that routes the specific paths to the appropriate port for that service.
// serviceHosts is a map of service name to URL to forward to. This allows
// mapping services to another destination.
// This is particularly useful to map app services to a hosted environment
// except the one under development. This allows local iteration with other
// service calls going to the cloud. Auth would have to be preserved of course.
func NewProxy(services []service.ServiceInterface, serviceHosts map[string]string) (*ServiceProxy, error) {
	hpl := hostPortLookup(serviceHosts)
	proxyMux := routes.NewMux()
	for _, svc := range services {
		svcMux := routes.NewMux()
		svc.RegisterRoutes(svcMux)
		destURL, err := hpl.Get(svc.Name(), svc.DefaultPort())
		if err != nil {
			return nil, fmt.Errorf("error processing destination url for %v: %w", svc.Name(), err)
		}
		serviceProxy := httputil.NewSingleHostReverseProxy(destURL)
		for _, rte := range svcMux.Routes() {
			proxyMux.Handle(rte.Pattern, serviceProxy)
		}
	}

	return &ServiceProxy{
		Port: project.DefaultStartingPort,
		Mux:  proxyMux,
	}, nil
}

func (sp *ServiceProxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	sp.Mux.ServeHTTP(rw, req)
}

// Run listens on Port until ctx is canceled.
func (sp *ServiceProxy) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", sp.Port),
		Handler: sp,
	}
	go func() {
		<-ctx.Done()
		srv.Shutdown(context.WithoutCancel(ctx))
	}()
	slog.Info(fmt.Sprintf("Running Proxy on %v", srv.Addr))
	err := srv.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

type hostPortLookup map[string]string

// Get checks overrides and returns the defaults if no override is present
func (hpl hostPortLookup) Get(service string, defaultPort int) (*url.URL, error) {
	urlString := ""
	if hpl != nil {
		urlString = hpl[service]
	}
	if urlString == "" {
		urlString = fmt.Sprintf("http://localhost:%d", defaultPort)
	}
	return url.Parse(urlString)
}
