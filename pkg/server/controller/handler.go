package controller

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/harvester/vm-dhcp-controller/pkg/allocator"
	"github.com/sirupsen/logrus"
)

func ListAllHandler(allocator allocator.Allocator) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		networkName := params["networkName"]
		set, err := allocator.ListAll(networkName)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "failed to list content of %s: %s", networkName, err.Error())
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
