package server

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"golang.org/x/sync/errgroup"
)

const defaultPort = 8080

const tmpDir = "/tmp/vm-dhcp-controller"

func NewServer(ctx context.Context) error {
	if err := createTmpDir(); err != nil {
		return err
	}
	return newServer(ctx, tmpDir)
}

func livenessProbeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "OK")
}

func readinessProbeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "OK")
}

func newServer(ctx context.Context, path string) error {
	defer os.RemoveAll(tmpDir)

	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", livenessProbeHandler)
	mux.HandleFunc("/readyz", readinessProbeHandler)
	mux.Handle("/files", http.FileServer(http.Dir(path)))

	srv := http.Server{
		Addr:    fmt.Sprintf(":%d", defaultPort),
		Handler: mux,
	}

	eg, _ := errgroup.WithContext(ctx)

	eg.Go(func() error {
		return srv.ListenAndServe()
	})

	eg.Go(func() error {
		<-ctx.Done()
		return srv.Shutdown(ctx)
	})

	return eg.Wait()
}

func createTmpDir() error {
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		return os.Mkdir(tmpDir, 0755)
	} else {
		return err
	}
}

// Address returns the address for vm-dhcp url. For local testing set env variable
// SVC_ADDRESS to point to a local endpoint
func Address() string {
	address := "harvester-vm-dhcp-controller.harvester-system.svc"
	if val := os.Getenv("SVC_ADDRESS"); val != "" {
		address = val
	}
	return address
}
