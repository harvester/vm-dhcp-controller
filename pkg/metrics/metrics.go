package metrics

import (
	"fmt"
	"net/http"
	"os"
	"strconv"

	log "github.com/sirupsen/logrus"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	LabelIPPoolName  = "ippool"
	LabelSubnet      = "subnet"
	LabelNetworkName = "network"
	LabelVMName      = "vm"
	LabelMacAddress  = "mac"
	LabelIPAddress   = "ip"
	LabelStatus      = "status"
)

type MetricsAllocator struct {
	kubevirtiphelperIPPoolUsed      *prometheus.GaugeVec
	kubevirtiphelperIPPoolAvailable *prometheus.GaugeVec
	kubevirtiphelperVmNetCfgStatus  *prometheus.GaugeVec
	registry                        *prometheus.Registry
}

func NewMetricsAllocator() *MetricsAllocator {
	m := &MetricsAllocator{
		kubevirtiphelperIPPoolUsed: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "kubevirtiphelper_ippool_used",
				Help: "Amount of IP addresses which are in use",
			},
			[]string{
				LabelIPPoolName,
				LabelSubnet,
				LabelNetworkName,
			},
		),
		kubevirtiphelperIPPoolAvailable: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "kubevirtiphelper_ippool_available",
				Help: "Amount of IP addresses which are available",
			},
			[]string{
				LabelIPPoolName,
				LabelSubnet,
				LabelNetworkName,
			},
		),
		kubevirtiphelperVmNetCfgStatus: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "kubevirtiphelper_vmnetcfg_status",
				Help: "Status of the vmnetcfg objects",
			},
			[]string{
				LabelVMName,
				LabelNetworkName,
				LabelMacAddress,
				LabelIPAddress,
				LabelStatus,
			},
		),
	}

	m.registry = prometheus.NewRegistry()
	m.registry.MustRegister(m.kubevirtiphelperIPPoolUsed)
	m.registry.MustRegister(m.kubevirtiphelperIPPoolAvailable)
	m.registry.MustRegister(m.kubevirtiphelperVmNetCfgStatus)

	return m
}

func (m *MetricsAllocator) UpdateIPPoolUsed(ippoolName string, subnet string, networkName string, used int) {
	m.kubevirtiphelperIPPoolUsed.With(prometheus.Labels{
		LabelIPPoolName:  ippoolName,
		LabelSubnet:      subnet,
		LabelNetworkName: networkName,
	}).Set(float64(used))
}

func (m *MetricsAllocator) UpdateIPPoolAvailable(ippoolName string, subnet string, networkName string, available int) {
	m.kubevirtiphelperIPPoolAvailable.With(prometheus.Labels{
		LabelIPPoolName:  ippoolName,
		LabelSubnet:      subnet,
		LabelNetworkName: networkName,
	}).Set(float64(available))
}

func (m *MetricsAllocator) DeleteIPPool(ippoolName string, subnet string, networkName string, used int) {
	m.kubevirtiphelperIPPoolUsed.Delete(prometheus.Labels{
		LabelIPPoolName:  ippoolName,
		LabelSubnet:      subnet,
		LabelNetworkName: networkName,
	})

	m.kubevirtiphelperIPPoolAvailable.Delete(prometheus.Labels{
		LabelIPPoolName:  ippoolName,
		LabelSubnet:      subnet,
		LabelNetworkName: networkName,
	})
}

func (m *MetricsAllocator) UpdateVmNetCfgStatus(vmName string, networkName string, macAddr string, ipAddr string, status string) {
	m.kubevirtiphelperVmNetCfgStatus.With(prometheus.Labels{
		LabelVMName:      vmName,
		LabelNetworkName: networkName,
		LabelMacAddress:  macAddr,
		LabelIPAddress:   ipAddr,
		LabelStatus:      status,
	}).Set(float64(1))
}

func (m *MetricsAllocator) DeleteVmNetCfgStatus(vmName string) {
	var vmnetCfgMetrics []prometheus.Labels
	var labelFound bool

	// gather all metrics so we make sure we delete all of them
	gatherer := prometheus.Gatherer(m.registry)
	mfs, err := gatherer.Gather()
	if err != nil {
		log.Errorf("(metrics.DeleteVmNetCfgStatus) error while gathering metrics for vm [%s]: %s",
			vmName, err.Error())

		return
	}
	for _, mf := range mfs {
		if mf.GetName() == "kubevirtiphelper_vmnetcfg_status" {
			for _, m := range mf.GetMetric() {
				labelFound = false
				pLabel := make(map[string]string)
				for _, l := range m.GetLabel() {
					pLabel[l.GetName()] = l.GetValue()
					if l.GetName() == LabelVMName && l.GetValue() == vmName {
						labelFound = true
					}
				}
				if labelFound {
					vmnetCfgMetrics = append(vmnetCfgMetrics, pLabel)
				}
			}
		}
	}

	// delete the metrics which contain the vm name
	for _, pl := range vmnetCfgMetrics {
		m.kubevirtiphelperVmNetCfgStatus.Delete(pl)
	}
}

func (m *MetricsAllocator) Run() {
	log.Infof("(metrics.Run) starting Metrics service")

	var metricsPort int

	metricsPort, err := strconv.Atoi(os.Getenv("METRICS_PORT"))
	if err != nil {
		metricsPort = 8080
	}

	listenAddress := fmt.Sprintf(":%d", metricsPort)
	http.Handle("/metrics", promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{Registry: m.registry}))
	log.Fatal(http.ListenAndServe(listenAddress, nil))
}

func New() *MetricsAllocator {
	return NewMetricsAllocator()
}
