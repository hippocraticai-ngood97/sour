package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/smw1218/sour/routes"
	"github.com/smw1218/sour/service"
)

type ServiceProxy struct {
	services []service.ServiceInterface
	mux      *routes.Mux
}

// NewProxy takes the full list of services and will create a reverse proxy
// that routes the specific paths to the appropriate port for that service.
// serviceHosts is a map of service name to URL to forward to. This allows
// mapping services to another destination.
// This is particularly useful to map app services to a hosted environment
// except the one under development. This allows local iteration with other
// service calls going to the cloud. Auth would have to be preserved of course.
// ServiceProxy is an http.Handler and is meant to be used with an http.Servers
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

	sp := &ServiceProxy{
		services: services,
		mux:      proxyMux,
	}
	return sp, nil
}

func (sp *ServiceProxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	sp.mux.ServeHTTP(rw, req)
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
