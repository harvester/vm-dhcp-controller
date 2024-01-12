package server

import (
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
	router *mux.Router
}

func NewHTTPServer() *HTTPServer {
	return &HTTPServer{
		router: mux.NewRouter(),
	}
}

func (s *HTTPServer) Register(routes []config.Route) {
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

	for _, route := range routes {
		if route.Prefix != "" {
			sr := s.router.PathPrefix(route.Prefix).Subrouter()
			for _, handle := range route.Handles {
				sr.Handle(handle.Path, handle.RegisterHandlerFunc(handle.Allocator))
			}
		} else {
			for _, handle := range route.Handles {
				s.router.Handle(handle.Path, handle.RegisterHandlerFunc(handle.Allocator))
			}
		}
	}
}

func (s *HTTPServer) Run() {
	logrus.Infof("Starting HTTP server")

	srv := &http.Server{
		Handler:      s.router,
		Addr:         fmt.Sprintf(":%d", defaultPort),
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	logrus.Infof("Listening on port: %d", defaultPort)

	logrus.Fatal(srv.ListenAndServe())
}
