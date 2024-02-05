package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"github.com/harvester/vm-dhcp-controller/pkg/config"
)

const defaultPort = 8080

type HTTPServer struct {
	*config.HTTPServerOptions
	srv    *http.Server
	router *mux.Router
}

func NewHTTPServer(httpServerOptions *config.HTTPServerOptions) *HTTPServer {
	return &HTTPServer{
		HTTPServerOptions: httpServerOptions,
		router:            mux.NewRouter(),
	}
}

func (s *HTTPServer) registerProbeHandlers() {
	s.router.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(map[string]bool{"ok": true}); err != nil {
			logrus.Fatal(err)
		}
	})
	s.router.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(map[string]bool{"ok": true}); err != nil {
			logrus.Fatal(err)
		}
	})
}

func (s *HTTPServer) RegisterControllerHandlers() {
	s.registerProbeHandlers()

	if s.DebugMode {
		s.router.Handle("/ipams/{networkName:.*}", listIPByNetworkHandler(s.IPAllocator))
		s.router.Handle("/caches/{networkName:.*}", listCacheByNetworkHandler(s.CacheAllocator))
	}

	s.router.Handle("/metrics", metricsHandler(s.MetricsAllocator))
}

func (s *HTTPServer) RegisterAgentHandlers() {
	s.registerProbeHandlers()

	if s.DebugMode {
		s.router.Handle("/leases", listLeaseHandler(s.DHCPAllocator))
	}
}

func (s *HTTPServer) Run() error {
	logrus.Info("Starting HTTP server")

	s.srv = &http.Server{
		Handler:      s.router,
		Addr:         fmt.Sprintf(":%d", defaultPort),
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	logrus.Infof("Listening on port: %d", defaultPort)

	return s.srv.ListenAndServe()
}

func (s *HTTPServer) stop(ctx context.Context) error {
	logrus.Info("Stopping HTTP server")

	return s.srv.Shutdown(ctx)
}

func Cleanup(ctx context.Context, srv *HTTPServer) <-chan error {
	errCh := make(chan error)

	go func() {
		<-ctx.Done()
		defer close(errCh)

		errCh <- srv.stop(ctx)
	}()

	return errCh
}
