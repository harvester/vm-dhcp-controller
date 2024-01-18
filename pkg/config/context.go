package config

import (
	"context"
	"fmt"

	harvesterv1 "github.com/harvester/harvester/pkg/apis/harvesterhci.io/v1beta1"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/schemes"
	"github.com/rancher/wrangler/pkg/start"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	kubevirtv1 "kubevirt.io/api/core/v1"

	"github.com/harvester/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
	"github.com/harvester/vm-dhcp-controller/pkg/cache"
	"github.com/harvester/vm-dhcp-controller/pkg/crd"
	"github.com/harvester/vm-dhcp-controller/pkg/dhcp"
	ctlcore "github.com/harvester/vm-dhcp-controller/pkg/generated/controllers/core"
	ctlcni "github.com/harvester/vm-dhcp-controller/pkg/generated/controllers/k8s.cni.cncf.io"
	ctlkubevirt "github.com/harvester/vm-dhcp-controller/pkg/generated/controllers/kubevirt.io"
	ctlnetwork "github.com/harvester/vm-dhcp-controller/pkg/generated/controllers/network.harvesterhci.io"
	"github.com/harvester/vm-dhcp-controller/pkg/ipam"
	"github.com/harvester/vm-dhcp-controller/pkg/metrics"
)

var (
	localSchemeBuilder = runtime.SchemeBuilder{
		v1alpha1.AddToScheme,
		harvesterv1.AddToScheme,
		kubevirtv1.AddToScheme,
	}
	AddToScheme = localSchemeBuilder.AddToScheme
	Scheme      = runtime.NewScheme()
)

func init() {
	utilruntime.Must(AddToScheme(Scheme))
	utilruntime.Must(schemes.AddToScheme(Scheme))
}

type RegisterFunc func(context.Context, *Management) error

type Image struct {
	Repository string
	Tag        string
}

func NewImage(repo, tag string) *Image {
	i := new(Image)
	i.Repository = repo
	i.Tag = tag
	return i
}

func (i *Image) String() string {
	return fmt.Sprintf("%s:%s", i.Repository, i.Tag)
}

type ControllerOptions struct {
	NoAgent                 bool
	AgentNamespace          string
	AgentImage              *Image
	AgentServiceAccountName string
	NoDHCP                  bool
}

type AgentOptions struct {
	DryRun         bool
	Nic            string
	KubeConfigPath string
	KubeContext    string
	IPPoolRef      types.NamespacedName
}

type HTTPServerOptions struct {
	DebugMode        bool
	CacheAllocator   *cache.CacheAllocator
	IPAllocator      *ipam.IPAllocator
	DHCPAllocator    *dhcp.DHCPAllocator
	MetricsAllocator *metrics.MetricsAllocator
}

type Management struct {
	ctx context.Context

	ControllerFactory controller.SharedControllerFactory

	HarvesterNetworkFactory *ctlnetwork.Factory

	CniFactory      *ctlcni.Factory
	CoreFactory     *ctlcore.Factory
	KubeVirtFactory *ctlkubevirt.Factory

	ClientSet *kubernetes.Clientset

	CacheAllocator   *cache.CacheAllocator
	IPAllocator      *ipam.IPAllocator
	MetricsAllocator *metrics.MetricsAllocator

	Options *ControllerOptions

	starters []start.Starter
}

func (s *Management) Start(threadiness int) error {
	return start.All(s.ctx, threadiness, s.starters...)
}

func (s *Management) Register(ctx context.Context, config *rest.Config, registerFuncList []RegisterFunc) error {
	if err := crd.Create(ctx, config); err != nil {
		return err
	}

	for _, f := range registerFuncList {
		if err := f(ctx, s); err != nil {
			return err
		}
	}

	return nil
}

func (s *Management) NewRecorder(componentName, namespace, nodeName string) record.EventRecorder {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(logrus.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: s.ClientSet.CoreV1().Events(namespace)})
	return eventBroadcaster.NewRecorder(Scheme, corev1.EventSource{Component: componentName, Host: nodeName})
}

func SetupManagement(ctx context.Context, restConfig *rest.Config, options *ControllerOptions) (*Management, error) {
	factory, err := controller.NewSharedControllerFactoryFromConfig(restConfig, Scheme)
	if err != nil {
		return nil, err
	}

	opts := &generic.FactoryOptions{
		SharedControllerFactory: factory,
	}

	management := &Management{
		ctx:     ctx,
		Options: options,
	}

	management.CacheAllocator = cache.NewCacheAllocator()
	management.IPAllocator = ipam.NewIPAllocator()
	management.MetricsAllocator = metrics.NewMetricsAllocator()

	harvesterNetwork, err := ctlnetwork.NewFactoryFromConfigWithOptions(restConfig, opts)
	if err != nil {
		return nil, err
	}
	management.HarvesterNetworkFactory = harvesterNetwork
	management.starters = append(management.starters, harvesterNetwork)

	core, err := ctlcore.NewFactoryFromConfigWithOptions(restConfig, opts)
	if err != nil {
		return nil, err
	}
	management.CoreFactory = core
	management.starters = append(management.starters, core)

	cni, err := ctlcni.NewFactoryFromConfigWithOptions(restConfig, opts)
	if err != nil {
		return nil, err
	}
	management.CniFactory = cni
	management.starters = append(management.starters, cni)

	kubevirt, err := ctlkubevirt.NewFactoryFromConfigWithOptions(restConfig, opts)
	if err != nil {
		return nil, err
	}
	management.KubeVirtFactory = kubevirt
	management.starters = append(management.starters, kubevirt)

	management.ClientSet, err = kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	return management, nil
}
