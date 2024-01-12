package agent

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/harvester/vm-dhcp-controller/pkg/allocator"
	"github.com/sirupsen/logrus"
)

func ListAllHandler(allocator allocator.Allocator) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		set, err := allocator.ListAll("")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "cannot list content: %s", err.Error())
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
