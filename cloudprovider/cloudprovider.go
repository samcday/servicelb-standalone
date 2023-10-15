package cloudprovider

import (
	"encoding/json"
	"github.com/rancher/wrangler/pkg/schemes"
	"io"
	"os"

	//"github.com/k3s-io/k3s/pkg/util"
	//"github.com/k3s-io/k3s/pkg/version"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/generated/controllers/apps"
	appsclient "github.com/rancher/wrangler/pkg/generated/controllers/apps/v1"
	"github.com/rancher/wrangler/pkg/generated/controllers/core"
	coreclient "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/generated/controllers/discovery"
	discoveryclient "github.com/rancher/wrangler/pkg/generated/controllers/discovery/v1"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/start"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	clientset "k8s.io/client-go/kubernetes"
	coregetter "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	cloudprovider "k8s.io/cloud-provider"
)

// Config describes externally-configurable cloud provider configuration.
// This is normally unmarshalled from a JSON config file.
type Config struct {
	LBImage     string `json:"lbImage"`
	LBNamespace string `json:"lbNamespace"`
	Rootless    bool   `json:"rootless"`
}

type k3s struct {
	Config

	client   kubernetes.Interface
	recorder record.EventRecorder

	processor      apply.Apply
	daemonsetCache appsclient.DaemonSetCache
	endpointsCache discoveryclient.EndpointSliceCache
	nodeCache      coreclient.NodeCache
	podCache       coreclient.PodCache
	workqueue      workqueue.RateLimitingInterface
}

var _ cloudprovider.Interface = &k3s{}

func Register() {
	cloudprovider.RegisterCloudProvider("servicelb-standalone", func(config io.Reader) (cloudprovider.Interface, error) {
		var err error
		k := k3s{
			Config: Config{
				LBImage:     DefaultLBImage,
				LBNamespace: DefaultLBNS,
			},
		}

		if config != nil {
			var bytes []byte
			bytes, err = io.ReadAll(config)
			if err == nil {
				err = json.Unmarshal(bytes, &k.Config)
			}
		}

		return &k, err
	})
}

func BuildControllerEventRecorder(k8s clientset.Interface, controllerName, namespace string) record.EventRecorder {
	logrus.Infof("Creating %s event broadcaster", controllerName)
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&coregetter.EventSinkImpl{Interface: k8s.CoreV1().Events(namespace)})
	nodeName := os.Getenv("NODE_NAME")
	return eventBroadcaster.NewRecorder(schemes.All, v1.EventSource{Component: controllerName, Host: nodeName})
}

func (k *k3s) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
	ctx := wait.ContextForChannel(stop)
	config := clientBuilder.ConfigOrDie(controllerName)
	k.client = kubernetes.NewForConfigOrDie(config)

	// Wrangler controller and caches are only needed if the load balancer controller is enabled.
	k.recorder = BuildControllerEventRecorder(k.client, controllerName, meta.NamespaceAll)
	coreFactory := core.NewFactoryFromConfigOrDie(config)
	k.nodeCache = coreFactory.Core().V1().Node().Cache()

	lbCoreFactory := core.NewFactoryFromConfigWithOptionsOrDie(config, &generic.FactoryOptions{Namespace: k.LBNamespace})
	lbAppsFactory := apps.NewFactoryFromConfigWithOptionsOrDie(config, &generic.FactoryOptions{Namespace: k.LBNamespace})
	lbDiscFactory := discovery.NewFactoryFromConfigOrDie(config)

	processor, err := apply.NewForConfig(config)
	if err != nil {
		logrus.Panicf("failed to create apply processor for %s: %v", controllerName, err)
	}
	k.processor = processor.WithDynamicLookup().WithCacheTypes(lbAppsFactory.Apps().V1().DaemonSet())
	k.daemonsetCache = lbAppsFactory.Apps().V1().DaemonSet().Cache()
	k.endpointsCache = lbDiscFactory.Discovery().V1().EndpointSlice().Cache()
	k.podCache = lbCoreFactory.Core().V1().Pod().Cache()
	k.workqueue = workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	if err := k.Register(ctx, coreFactory.Core().V1().Node(), lbCoreFactory.Core().V1().Pod(), lbDiscFactory.Discovery().V1().EndpointSlice()); err != nil {
		logrus.Panicf("failed to register %s handlers: %v", controllerName, err)
	}

	if err := start.All(ctx, 1, coreFactory, lbCoreFactory, lbAppsFactory, lbDiscFactory); err != nil {
		logrus.Panicf("failed to start %s controllers: %v", controllerName, err)
	}
}

func (k *k3s) Instances() (cloudprovider.Instances, bool) {
	return nil, false
}

func (k *k3s) InstancesV2() (cloudprovider.InstancesV2, bool) {
	return nil, false
}

func (k *k3s) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return k, true
}

func (k *k3s) Zones() (cloudprovider.Zones, bool) {
	return nil, false
}

func (k *k3s) Clusters() (cloudprovider.Clusters, bool) {
	return nil, false
}

func (k *k3s) Routes() (cloudprovider.Routes, bool) {
	return nil, false
}

func (k *k3s) ProviderName() string {
	return "servicelb-standalone"
}

func (k *k3s) HasClusterID() bool {
	return false
}
