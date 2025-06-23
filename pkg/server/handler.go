package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"github.com/harvester/vm-dhcp-controller/pkg/cache"
	"github.com/harvester/vm-dhcp-controller/pkg/dhcp"
	"github.com/harvester/vm-dhcp-controller/pkg/ipam"
	"github.com/harvester/vm-dhcp-controller/pkg/metrics"
)

func listIPByNetworkHandler(ipAllocator *ipam.IPAllocator) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		networkName := params["networkName"]
		set, err := ipAllocator.ListAll(networkName)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprintf(w, "failed to list ipam of %s: %s", networkName, err.Error())
			return
		}
		payload, err := json.Marshal(set)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(payload); err != nil {
			logrus.Error(err)
		}
	})
}

func listCacheByNetworkHandler(cacheAllocator *cache.CacheAllocator) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		networkName := params["networkName"]
		set, err := cacheAllocator.ListAll(networkName)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprintf(w, "failed to list cache of %s: %s", networkName, err.Error())
			return
		}
		payload, err := json.Marshal(set)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(payload); err != nil {
			logrus.Error(err)
		}
	})
}

func listLeaseHandler(dhcpAllocator *dhcp.DHCPAllocator) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		set, err := dhcpAllocator.ListAll("")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprintf(w, "cannot list leases: %s", err.Error())
			return
		}
		payload, err := json.Marshal(set)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(payload); err != nil {
			logrus.Error(err)
		}
	})
}

func metricsHandler(metricsAllocator *metrics.MetricsAllocator) http.Handler {
	return metricsAllocator.GetHTTPHandler()
}
