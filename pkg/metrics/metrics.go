package metrics

import (
	"net/http"

	"github.com/sirupsen/logrus"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	LabelIPPoolName   = "ippool"
	LabelCIDR         = "cidr"
	LabelNetworkName  = "network"
	LabelVmNetCfgName = "vmnetcfg"
	LabelMACAddress   = "mac"
	LabelIPAddress    = "ip"
	LabelStatus       = "status"
)

type MetricsAllocator struct {
	ipPoolUsed      *prometheus.GaugeVec
	ipPoolAvailable *prometheus.GaugeVec
	vmNetCfgStatus  *prometheus.GaugeVec
	registry        *prometheus.Registry
}

func NewMetricsAllocator() *MetricsAllocator {
	metricsAllocator := &MetricsAllocator{
		ipPoolUsed: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vmdhcpcontroller_ippool_used",
				Help: "Amount of IP addresses which are in use",
			},
			[]string{
				LabelIPPoolName,
				LabelCIDR,
				LabelNetworkName,
			},
		),
		ipPoolAvailable: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vmdhcpcontroller_ippool_available",
				Help: "Amount of IP addresses which are available",
			},
			[]string{
				LabelIPPoolName,
				LabelCIDR,
				LabelNetworkName,
			},
		),
		vmNetCfgStatus: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vmdhcpcontroller_vmnetcfg_status",
				Help: "Status of the vmnetcfg objects",
			},
			[]string{
				LabelVmNetCfgName,
				LabelNetworkName,
				LabelMACAddress,
				LabelIPAddress,
				LabelStatus,
			},
		),
	}

	metricsAllocator.registry = prometheus.NewRegistry()
	metricsAllocator.registry.MustRegister(metricsAllocator.ipPoolUsed)
	metricsAllocator.registry.MustRegister(metricsAllocator.ipPoolAvailable)
	metricsAllocator.registry.MustRegister(metricsAllocator.vmNetCfgStatus)

	return metricsAllocator
}

func (a *MetricsAllocator) UpdateIPPoolUsed(name string, cidr string, networkName string, used int) {
	a.ipPoolUsed.With(prometheus.Labels{
		LabelIPPoolName:  name,
		LabelCIDR:        cidr,
		LabelNetworkName: networkName,
	}).Set(float64(used))
}

func (a *MetricsAllocator) UpdateIPPoolAvailable(name string, cidr string, networkName string, available int) {
	a.ipPoolAvailable.With(prometheus.Labels{
		LabelIPPoolName:  name,
		LabelCIDR:        cidr,
		LabelNetworkName: networkName,
	}).Set(float64(available))
}

func (a *MetricsAllocator) DeleteIPPool(name string, cidr string, networkName string) {
	a.ipPoolUsed.Delete(prometheus.Labels{
		LabelIPPoolName:  name,
		LabelCIDR:        cidr,
		LabelNetworkName: networkName,
	})

	a.ipPoolAvailable.Delete(prometheus.Labels{
		LabelIPPoolName:  name,
		LabelCIDR:        cidr,
		LabelNetworkName: networkName,
	})
}

func (a *MetricsAllocator) UpdateVmNetCfgStatus(name, networkName, macAddress, ipAddress, status string) {
	a.vmNetCfgStatus.With(prometheus.Labels{
		LabelVmNetCfgName: name,
		LabelNetworkName:  networkName,
		LabelMACAddress:   macAddress,
		LabelIPAddress:    ipAddress,
		LabelStatus:       status,
	}).Set(float64(1))
}

func (a *MetricsAllocator) DeleteVmNetCfgStatus(name string) {
	var vmNetCfgMetrics []prometheus.Labels

	// Gather all metrics so we make sure we delete all of them
	gatherer := prometheus.Gatherer(a.registry)
	mfs, err := gatherer.Gather()
	if err != nil {
		logrus.Errorf("(metrics.DeleteVmNetCfgStatus) error while gathering metrics for vmnetcfg [%s]: %s", name, err.Error())
		return
	}

	for _, mf := range mfs {
		if mf.GetName() == "vmdhcpcontroller_vmnetcfg_status" {
			for _, m := range mf.GetMetric() {
				var found bool
				pLabel := make(map[string]string)
				for _, l := range m.GetLabel() {
					pLabel[l.GetName()] = l.GetValue()
					if l.GetName() == LabelVmNetCfgName && l.GetValue() == name {
						found = true
					}
				}
				if found {
					vmNetCfgMetrics = append(vmNetCfgMetrics, pLabel)
				}
			}
		}
	}

	// Delete the metrics which contain the vmnetcfg's name
	for _, pl := range vmNetCfgMetrics {
		a.vmNetCfgStatus.Delete(pl)
	}
}

func (a *MetricsAllocator) GetHTTPHandler() http.Handler {
	return promhttp.HandlerFor(
		a.registry,
		promhttp.HandlerOpts{
			Registry: a.registry,
		},
	)
}

func New() *MetricsAllocator {
	return NewMetricsAllocator()
}
