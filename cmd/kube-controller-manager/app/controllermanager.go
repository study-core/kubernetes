/*
Copyright 2014 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package app implements a server that runs a set of active
// components.  This includes replication controllers, service endpoints and
// nodes.
//
package app

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/healthz"
	"k8s.io/apiserver/pkg/server/mux"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/apiserver/pkg/util/term"
	cacheddiscovery "k8s.io/client-go/discovery/cached"
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/metadata/metadatainformer"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"
	cloudprovider "k8s.io/cloud-provider"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/cli/globalflag"
	"k8s.io/component-base/version"
	"k8s.io/component-base/version/verflag"
	"k8s.io/klog"
	genericcontrollermanager "k8s.io/kubernetes/cmd/controller-manager/app"
	"k8s.io/kubernetes/cmd/kube-controller-manager/app/config"
	"k8s.io/kubernetes/cmd/kube-controller-manager/app/options"
	"k8s.io/kubernetes/pkg/controller"
	kubectrlmgrconfig "k8s.io/kubernetes/pkg/controller/apis/config"
	serviceaccountcontroller "k8s.io/kubernetes/pkg/controller/serviceaccount"
	"k8s.io/kubernetes/pkg/features"
	"k8s.io/kubernetes/pkg/serviceaccount"
	"k8s.io/kubernetes/pkg/util/configz"
	utilflag "k8s.io/kubernetes/pkg/util/flag"
)

const (
	// ControllerStartJitter is the Jitter used when starting controller managers
	ControllerStartJitter = 1.0
	// ConfigzName is the name used for register kube-controller manager /configz, same with GroupName.
	ConfigzName = "kubecontrollermanager.config.k8s.io"
)

// ControllerLoopMode is the kube-controller-manager's mode of running controller loops that are cloud provider dependent
type ControllerLoopMode int

const (
	// IncludeCloudLoops means the kube-controller-manager include the controller loops that are cloud provider dependent
	//
	// todo IncludeCloudLoops 意味着 kube-controller-manager 包括依赖于云提供程序的控制器循环
	IncludeCloudLoops ControllerLoopMode = iota

	// ExternalLoops means the kube-controller-manager exclude the controller loops that are cloud provider dependent
	//
	// todo ExternalLoops 意味着 kube-controller-manager 排除依赖于云提供程序的控制器循环
	ExternalLoops
)

// NewControllerManagerCommand creates a *cobra.Command object with default parameters
func NewControllerManagerCommand() *cobra.Command {
	s, err := options.NewKubeControllerManagerOptions()
	if err != nil {
		klog.Fatalf("unable to initialize command options: %v", err)
	}

	cmd := &cobra.Command{
		Use: "kube-controller-manager",
		Long: `The Kubernetes controller manager is a daemon that embeds
the core control loops shipped with Kubernetes. In applications of robotics and
automation, a control loop is a non-terminating loop that regulates the state of
the system. In Kubernetes, a controller is a control loop that watches the shared
state of the cluster through the apiserver and makes changes attempting to move the
current state towards the desired state. Examples of controllers that ship with
Kubernetes today are the replication controller, endpoints controller, namespace
controller, and serviceaccounts controller.`,

		// Command 对象的回调函数
		Run: func(cmd *cobra.Command, args []string) {
			verflag.PrintAndExitIfRequested()
			utilflag.PrintFlags(cmd.Flags())

			// 返回所有已知的 controller 的 name
			c, err := s.Config(KnownControllers(), ControllersDisabledByDefault.List())
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}


			// todo 运行 Kube Controller Manager 的 Run() 方法
			if err := Run(c.Complete(), wait.NeverStop); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		},
	}

	fs := cmd.Flags()
	// todo 加载 Controller的命令行参数集
	namedFlagSets := s.Flags(KnownControllers(), ControllersDisabledByDefault.List())
	verflag.AddFlags(namedFlagSets.FlagSet("global"))
	globalflag.AddGlobalFlags(namedFlagSets.FlagSet("global"), cmd.Name())
	registerLegacyGlobalFlags(namedFlagSets)
	for _, f := range namedFlagSets.FlagSets {
		fs.AddFlagSet(f)
	}
	usageFmt := "Usage:\n  %s\n"
	cols, _, _ := term.TerminalSize(cmd.OutOrStdout())
	cmd.SetUsageFunc(func(cmd *cobra.Command) error {
		fmt.Fprintf(cmd.OutOrStderr(), usageFmt, cmd.UseLine())
		cliflag.PrintSections(cmd.OutOrStderr(), namedFlagSets, cols)
		return nil
	})
	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n"+usageFmt, cmd.Long, cmd.UseLine())
		cliflag.PrintSections(cmd.OutOrStdout(), namedFlagSets, cols)
	})

	return cmd
}

// ResyncPeriod returns a function which generates a duration each time it is
// invoked; this is so that multiple controllers don't get into lock-step and all
// hammer the apiserver with list requests simultaneously.
func ResyncPeriod(c *config.CompletedConfig) func() time.Duration {
	return func() time.Duration {
		factor := rand.Float64() + 1
		return time.Duration(float64(c.ComponentConfig.Generic.MinResyncPeriod.Nanoseconds()) * factor)
	}
}

// Run runs the KubeControllerManagerOptions.  This should never exit.
//
// 运行运行 `KubeControllerManagerOptions` 这永远都不会退出
func Run(c *config.CompletedConfig, stopCh <-chan struct{}) error {

	// To help debugging, immediately log version
	//
	// 为了帮助调试，请立即记录版本
	klog.Infof("Version: %+v", version.Get())


	// 读取配置信息
	if cfgz, err := configz.New(ConfigzName); err == nil {
		cfgz.Set(c.ComponentConfig)
	} else {
		klog.Errorf("unable to register configz: %v", err)
	}

	// Setup any healthz checks we will want to use.
	//
	// 设置我们将要使用的所有healthz检查
	var checks []healthz.HealthChecker
	var electionChecker *leaderelection.HealthzAdaptor
	if c.ComponentConfig.Generic.LeaderElection.LeaderElect {
		electionChecker = leaderelection.NewLeaderHealthzAdaptor(time.Second * 20)
		checks = append(checks, electionChecker)
	}

	// Start the controller manager HTTP server
	// unsecuredMux is the handler for these controller *after* authn/authz filters have been applied
	var unsecuredMux *mux.PathRecorderMux
	if c.SecureServing != nil {
		unsecuredMux = genericcontrollermanager.NewBaseHandler(&c.ComponentConfig.Generic.Debugging, checks...)
		handler := genericcontrollermanager.BuildHandlerChain(unsecuredMux, &c.Authorization, &c.Authentication)
		// TODO: handle stoppedCh returned by c.SecureServing.Serve
		if _, err := c.SecureServing.Serve(handler, 0, stopCh); err != nil {
			return err
		}
	}
	if c.InsecureServing != nil {
		unsecuredMux = genericcontrollermanager.NewBaseHandler(&c.ComponentConfig.Generic.Debugging, checks...)
		insecureSuperuserAuthn := server.AuthenticationInfo{Authenticator: &server.InsecureSuperuser{}}
		handler := genericcontrollermanager.BuildHandlerChain(unsecuredMux, nil, &insecureSuperuserAuthn)
		if err := c.InsecureServing.Serve(handler, 0, stopCh); err != nil {
			return err
		}
	}

	run := func(ctx context.Context) {
		rootClientBuilder := controller.SimpleControllerClientBuilder{
			ClientConfig: c.Kubeconfig,
		}
		var clientBuilder controller.ControllerClientBuilder
		if c.ComponentConfig.KubeCloudShared.UseServiceAccountCredentials {
			if len(c.ComponentConfig.SAController.ServiceAccountKeyFile) == 0 {
				// It's possible another controller process is creating the tokens for us.
				// If one isn't, we'll timeout and exit when our client builder is unable to create the tokens.
				klog.Warningf("--use-service-account-credentials was specified without providing a --service-account-private-key-file")
			}

			if shouldTurnOnDynamicClient(c.Client) {
				klog.V(1).Infof("using dynamic client builder")
				//Dynamic builder will use TokenRequest feature and refresh service account token periodically
				clientBuilder = controller.NewDynamicClientBuilder(
					restclient.AnonymousClientConfig(c.Kubeconfig),
					c.Client.CoreV1(),
					"kube-system")
			} else {
				klog.V(1).Infof("using legacy client builder")
				clientBuilder = controller.SAControllerClientBuilder{
					ClientConfig:         restclient.AnonymousClientConfig(c.Kubeconfig),
					CoreClient:           c.Client.CoreV1(),
					AuthenticationClient: c.Client.AuthenticationV1(),
					Namespace:            "kube-system",
				}
			}
		} else {
			clientBuilder = rootClientBuilder
		}
		controllerContext, err := CreateControllerContext(c, rootClientBuilder, clientBuilder, ctx.Done())
		if err != nil {
			klog.Fatalf("error building controller context: %v", err)
		}
		saTokenControllerInitFunc := serviceAccountTokenControllerStarter{rootClientBuilder: rootClientBuilder}.startServiceAccountTokenController

		// todo 启动各个 Controller
		if err := StartControllers(controllerContext, saTokenControllerInitFunc, NewControllerInitializers(controllerContext.LoopMode), unsecuredMux); err != nil {
			klog.Fatalf("error starting controllers: %v", err)
		}

		// todo
		//		所有需要监控资源变化情况的调用均推荐使用 Informer
		//		Informer 提供了基于事件通知的只读缓存机制，可以注册资源变化的回调函数，并可以极大减少 API 的调用
		controllerContext.InformerFactory.Start(controllerContext.Stop)
		controllerContext.ObjectOrMetadataInformerFactory.Start(controllerContext.Stop)
		close(controllerContext.InformersStarted)

		// 阻塞
		select {}
	}

	// TODO
	// 		在启动时设置 --leader-elect=true 后，controller manager 会使用多节点选主的方式选择主节点
	// 		只有主节点才会调用 StartControllers() 启动所有控制器，而其他从节点则仅执行选主算法

	// 如果不是主节点, 则 不启动各个 Controller
	if !c.ComponentConfig.Generic.LeaderElection.LeaderElect {
		run(context.TODO())
		panic("unreachable")
	}

	id, err := os.Hostname()
	if err != nil {
		return err
	}

	// add a uniquifier so that two processes on the same host don't accidentally both become active
	id = id + "_" + string(uuid.NewUUID())

	rl, err := resourcelock.New(c.ComponentConfig.Generic.LeaderElection.ResourceLock,
		c.ComponentConfig.Generic.LeaderElection.ResourceNamespace,
		c.ComponentConfig.Generic.LeaderElection.ResourceName,
		c.LeaderElectionClient.CoreV1(),
		c.LeaderElectionClient.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity:      id,
			EventRecorder: c.EventRecorder,
		})
	if err != nil {
		klog.Fatalf("error creating lock: %v", err)
	}


	// 选举 主节点 (controller的主节点 ??)
	leaderelection.RunOrDie(context.TODO(), leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: c.ComponentConfig.Generic.LeaderElection.LeaseDuration.Duration,
		RenewDeadline: c.ComponentConfig.Generic.LeaderElection.RenewDeadline.Duration,
		RetryPeriod:   c.ComponentConfig.Generic.LeaderElection.RetryPeriod.Duration,

		// 选举回调,  只有主节点才会做 回调哦 (leaderelection.go中)
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: run, // 注册启动各个 Controller的 run()
			OnStoppedLeading: func() {
				klog.Fatalf("leaderelection lost")
			},
		},
		WatchDog: electionChecker,
		Name:     "kube-controller-manager",
	})
	// todo 尼玛， 这是什么骚操作 ?? `无法达到` ??
	panic("unreachable")
}

// ControllerContext defines the context object for controller
//
// 定义了 关于 Controller 的上下文信息
type ControllerContext struct {
	// ClientBuilder will provide a client for this controller to use
	ClientBuilder controller.ControllerClientBuilder

	// InformerFactory gives access to informers for the controller.
	InformerFactory informers.SharedInformerFactory

	// ObjectOrMetadataInformerFactory gives access to informers for typed resources
	// and dynamic resources by their metadata. All generic controllers currently use
	// object metadata - if a future controller needs access to the full object this
	// would become GenericInformerFactory and take a dynamic client.
	ObjectOrMetadataInformerFactory controller.InformerFactory

	// ComponentConfig provides access to init options for a given controller
	//
	// ComponentConfig 提供对 给定控制器 的初始化选项的访问
	ComponentConfig kubectrlmgrconfig.KubeControllerManagerConfiguration

	// DeferredDiscoveryRESTMapper is a RESTMapper that will defer
	// initialization of the RESTMapper until the first mapping is
	// requested.
	RESTMapper *restmapper.DeferredDiscoveryRESTMapper

	// AvailableResources is a map listing currently available resources
	AvailableResources map[schema.GroupVersionResource]bool

	// Cloud is the cloud provider interface for the controllers to use.
	// It must be initialized and ready to use.
	Cloud cloudprovider.Interface

	// Control for which control loops to be run
	// IncludeCloudLoops is for a kube-controller-manager running all loops
	// ExternalLoops is for a kube-controller-manager running with a cloud-controller-manager
	LoopMode ControllerLoopMode

	// Stop is the stop channel
	Stop <-chan struct{}

	// InformersStarted is closed after all of the controllers have been initialized and are running.  After this point it is safe,
	// for an individual controller to start the shared informers. Before it is closed, they should not.
	InformersStarted chan struct{}

	// ResyncPeriod generates a duration each time it is invoked; this is so that
	// multiple controllers don't get into lock-step and all hammer the apiserver
	// with list requests simultaneously.
	ResyncPeriod func() time.Duration
}

// IsControllerEnabled checks if the context's controllers enabled or not
//
// IsControllerEnabled 检查上下文的控制器 是否可以启用
func (c ControllerContext) IsControllerEnabled(name string) bool {
	return genericcontrollermanager.IsControllerEnabled(name, ControllersDisabledByDefault, c.ComponentConfig.Generic.Controllers)
}

// InitFunc is used to launch a particular controller.  It may run additional "should I activate checks".
// Any error returned will cause the controller process to `Fatal`
// The bool indicates whether the controller was enabled.
//
// todo InitFunc
// 	    用于启动特定的 Controller。 它可能会运行其他“我应该激活检查”。
// 	    返回的任何错误都将导致控制器进程“致命”。
// 	    布尔值指示是否启用了控制器。
type InitFunc func(ctx ControllerContext) (debuggingHandler http.Handler, enabled bool, err error)

// KnownControllers returns all known controllers's name
//
// 返回所有已知的 controller 的 name
func KnownControllers() []string {
	ret := sets.StringKeySet(NewControllerInitializers(IncludeCloudLoops))

	// add "special" controllers that aren't initialized normally.  These controllers cannot be initialized
	// using a normal function.  The only known special case is the SA token controller which *must* be started
	// first to ensure that the SA tokens for future controllers will exist.  Think very carefully before adding
	// to this list.
	ret.Insert(
		saTokenControllerName,
	)

	return ret.List()
}

// ControllersDisabledByDefault is the set of controllers which is disabled by default
//
// todo ControllersDisabledByDefault 是默认情况下禁用的控制器集
//
// 之前启动各个 controller时，决定了 默认时 启动 还是 禁止的选项在这里
var ControllersDisabledByDefault = sets.NewString(
	"bootstrapsigner",
	"tokencleaner",
)

const (
	saTokenControllerName = "serviceaccount-token"
)

// NewControllerInitializers is a public map of named controller groups (you can start more than one in an init func)
// paired to their InitFunc.  This allows for structured downstream composition and subdivision.
//
// todo NewControllerInitializers
// 		是已命名 COntroller 组（您可以在init函数中启动多个）与它们的InitFunc配对的公共映射。
//		这允许结构化的下游组成和细分。
func NewControllerInitializers(loopMode ControllerLoopMode) map[string]InitFunc {

	// 收集 所有 COntroller 组件的 map
	controllers := map[string]InitFunc{}

	// todo 收集所有 启动 各个  Controller 组件的 `startXxx`函数，
	// todo 这些组件共同 组成了 `kube-controller-manager`

	// todo kube-controller-manager 由一系列的控制器组成，这些控制器可以划分为三组
	//		必须启动的 | 可选的(默认启动的) | 可选的(默认禁止的)

	controllers["endpoint"] = startEndpointController							// 必须启动,  管理基于选择器的服务端点
	controllers["endpointslice"] = startEndpointSliceController
	controllers["replicationcontroller"] = startReplicationController			// 必须启动,  它只是ReplicaSetController的包装, 负责将系统中存储的ReplicationController对象与实际运行的Pod进行同步
	controllers["podgc"] = startPodGCController									// 必须启动,  管理 pod 的资源回收 ??
	controllers["resourcequota"] = startResourceQuotaController					// 必须启动,  负责跟踪系统中的配额使用状态
	controllers["namespace"] = startNamespaceController							// 必须启动,  负责命名空间管理
	controllers["serviceaccount"] = startServiceAccountController				// 必须启动,  Namespaces 中的 ServiceAccounts Controller Manager 服务帐户对象
	controllers["garbagecollector"] = startGarbageCollectorController			// 必须启动,  负责系统资源回收
	controllers["daemonset"] = startDaemonSetController							// 必须启动,  负责守护进程 集管理
	controllers["job"] = startJobController										// 必须启动,  负责job作业管理
	controllers["deployment"] = startDeploymentController						// 必须启动,  部署管理, 负责将系统中存储的Deployment对象与实际运行的副本集和Pod进行同步.
	controllers["replicaset"] = startReplicaSetController						// 必须启动,  副本集管理, 负责将系统中存储的Replica Set对象与实际运行的Pod进行同步
	controllers["horizontalpodautoscaling"] = startHPAController				// 必须启动,  资源[水平自动伸缩]管理, 根据 `CPU利用率` 调节 rc 和 deployment 和 rs等等
	controllers["disruption"] = startDisruptionController						// 必须启动,  中断 控制器
	controllers["statefulset"] = startStatefulSetController						// 必须启动,  有状态服务集控制器
	controllers["cronjob"] = startCronJobController								// 必须启动,  定时任务控制器
	controllers["csrsigning"] = startCSRSigningController						// 必须启动,  Certificate Signing Request,证书注册请求, CSR签名控制器
	controllers["csrapproving"] = startCSRApprovingController					// 必须启动,  CSR批准控制器
	controllers["csrcleaner"] = startCSRCleanerController						// 必须启动,  CSR旧证书回收(GC)控制器
	controllers["ttl"] = startTTLController										// 必须启动,  Time To Live (存活时间)控制器


	controllers["bootstrapsigner"] = startBootstrapSignerController				// 可选(默认禁止的), BootstrapSigner 控制器也会使用启动引导令牌为这类对象生成签名信息.
	controllers["tokencleaner"] = startTokenCleanerController					// 可选(默认禁止的), TokenCleaner 控制器能够删除过期的 启动引导令牌
	controllers["nodeipam"] = startNodeIpamController							//   主要处理Node的IPAM地址相关.
	controllers["nodelifecycle"] = startNodeLifecycleController					//   处理Node的整个生命周期.

	// todo IncludeCloudLoops 意味着 kube-controller-manager 包括依赖于云提供程序的控制器循环
	if loopMode == IncludeCloudLoops {

		// 在 Kubernetes 启用 Cloud Provider 的时候才需要，用来配合云服务提供商的控制，也包括一系列的控制器
		//
		// 属于 cloud-controller-manager

		controllers["service"] = startServiceController								// 可选的(默认启动的), 管理 service 的控制器
		controllers["route"] = startRouteController									// 可选的(默认启动的), 管理 route 的控制器 ??
		controllers["cloud-node-lifecycle"] = startCloudNodeLifecycleController     // 管理 云节点 的生命周期 ??
		// TODO: volume controller into the IncludeCloudLoops only set.
	}


	// 下面三个时和 存储相关的 controller
	controllers["persistentvolume-binder"] = startPersistentVolumeBinderController  // 可选的(默认启动的), 就是PV Controller，主要负责pv和pvc的生命周期以及状态的切换
	controllers["attachdetach"] = startAttachDetachController						// 可选的(默认启动的), 简称AD Controller，主要处理真实的与volume相关的操作
	controllers["persistentvolume-expander"] = startVolumeExpandController			//                  主要负责volume的扩容操作


	controllers["clusterrole-aggregation"] = startClusterRoleAggregrationController // 是用于组合集群角色的控制器.
	controllers["pvc-protection"] = startPVCProtectionController				    // PVC 保护相关的 控制器
	controllers["pv-protection"] = startPVProtectionController						// PV 保护相关的 控制器
	controllers["ttl-after-finished"] = startTTLAfterFinishedController             // 用于在任务结束后, 让TTL控制器清理Pod和Jobs
	controllers["root-ca-cert-publisher"] = startRootCACertPublisher

	return controllers
}

// GetAvailableResources gets the map which contains all available resources of the apiserver
// TODO: In general, any controller checking this needs to be dynamic so
// users don't have to restart their controller manager if they change the apiserver.
// Until we get there, the structure here needs to be exposed for the construction of a proper ControllerContext.
func GetAvailableResources(clientBuilder controller.ControllerClientBuilder) (map[schema.GroupVersionResource]bool, error) {
	client := clientBuilder.ClientOrDie("controller-discovery")
	discoveryClient := client.Discovery()
	_, resourceMap, err := discoveryClient.ServerGroupsAndResources()
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to get all supported resources from server: %v", err))
	}
	if len(resourceMap) == 0 {
		return nil, fmt.Errorf("unable to get any supported resources from server")
	}

	allResources := map[schema.GroupVersionResource]bool{}
	for _, apiResourceList := range resourceMap {
		version, err := schema.ParseGroupVersion(apiResourceList.GroupVersion)
		if err != nil {
			return nil, err
		}
		for _, apiResource := range apiResourceList.APIResources {
			allResources[version.WithResource(apiResource.Name)] = true
		}
	}

	return allResources, nil
}

// CreateControllerContext creates a context struct containing references to resources needed by the
// controllers such as the cloud provider and clientBuilder. rootClientBuilder is only used for
// the shared-informers client and token controller.
func CreateControllerContext(s *config.CompletedConfig, rootClientBuilder, clientBuilder controller.ControllerClientBuilder, stop <-chan struct{}) (ControllerContext, error) {
	versionedClient := rootClientBuilder.ClientOrDie("shared-informers")
	sharedInformers := informers.NewSharedInformerFactory(versionedClient, ResyncPeriod(s)())

	metadataClient := metadata.NewForConfigOrDie(rootClientBuilder.ConfigOrDie("metadata-informers"))
	metadataInformers := metadatainformer.NewSharedInformerFactory(metadataClient, ResyncPeriod(s)())

	// If apiserver is not running we should wait for some time and fail only then. This is particularly
	// important when we start apiserver and controller manager at the same time.
	if err := genericcontrollermanager.WaitForAPIServer(versionedClient, 10*time.Second); err != nil {
		return ControllerContext{}, fmt.Errorf("failed to wait for apiserver being healthy: %v", err)
	}

	// Use a discovery client capable of being refreshed.
	discoveryClient := rootClientBuilder.ClientOrDie("controller-discovery")
	cachedClient := cacheddiscovery.NewMemCacheClient(discoveryClient.Discovery())
	restMapper := restmapper.NewDeferredDiscoveryRESTMapper(cachedClient)
	go wait.Until(func() {
		restMapper.Reset()
	}, 30*time.Second, stop)

	availableResources, err := GetAvailableResources(rootClientBuilder)
	if err != nil {
		return ControllerContext{}, err
	}

	cloud, loopMode, err := createCloudProvider(s.ComponentConfig.KubeCloudShared.CloudProvider.Name, s.ComponentConfig.KubeCloudShared.ExternalCloudVolumePlugin,
		s.ComponentConfig.KubeCloudShared.CloudProvider.CloudConfigFile, s.ComponentConfig.KubeCloudShared.AllowUntaggedCloud, sharedInformers)
	if err != nil {
		return ControllerContext{}, err
	}

	ctx := ControllerContext{
		ClientBuilder:                   clientBuilder,
		InformerFactory:                 sharedInformers,
		ObjectOrMetadataInformerFactory: controller.NewInformerFactory(sharedInformers, metadataInformers),
		ComponentConfig:                 s.ComponentConfig,
		RESTMapper:                      restMapper,
		AvailableResources:              availableResources,
		Cloud:                           cloud,
		LoopMode:                        loopMode,
		Stop:                            stop,
		InformersStarted:                make(chan struct{}),
		ResyncPeriod:                    ResyncPeriod(s),
	}
	return ctx, nil
}

// StartControllers starts a set of controllers with a specified ControllerContext
//
// StartControllers:
//		使用指定的ControllerContext启动一组控制器
//
//
// 在启动时设置 `--leader-elect=true` 后，controller manager 会使用多节点选主的方式选择主节点.
// 只有主节点才会调用 StartControllers() 启动所有控制器，而其他从节点则仅执行选主算法.
func StartControllers(ctx ControllerContext, startSATokenController InitFunc, controllers map[string]InitFunc, unsecuredMux *mux.PathRecorderMux) error {

	// Always start the SA token controller first using a full-power client, since it needs to mint tokens for the rest
	// If this fails, just return here and fail since other controllers won't be able to get credentials.
	//
	// 始终首先使用全功能客户端启动 SA (ServiceAccount) 令牌controller，因为它需要为 其余的 Controller 铸造令牌。
	// 如果失败，则返回此处失败，因为其他控制器将无法获得凭据。
	if _, _, err := startSATokenController(ctx); err != nil {
		return err
	}

	// Initialize the cloud provider with a reference to the clientBuilder only after token controller
	// has started in case the cloud provider uses the client builder.
	if ctx.Cloud != nil {
		ctx.Cloud.Initialize(ctx.ClientBuilder, ctx.Stop)
	}

	// todo 逐个 启动 各个 controller
	for controllerName, initFn := range controllers {

		// 如果 不允许启用, 则跳过该 Controller
		if !ctx.IsControllerEnabled(controllerName) {
			klog.Warningf("%q is disabled", controllerName)
			continue
		}

		// 休眠下 (抖动时间)
		time.Sleep(wait.Jitter(ctx.ComponentConfig.Generic.ControllerStartInterval.Duration, ControllerStartJitter))

		klog.V(1).Infof("Starting %q", controllerName)

		/** todo 启动 Controller 了 */
		debugHandler, started, err := initFn(ctx)
		if err != nil {
			klog.Errorf("Error starting %q", controllerName)
			return err
		}
		// 如果启动不成功的话
		if !started {
			klog.Warningf("Skipping %q", controllerName)
			continue
		}

		// debugger
		if debugHandler != nil && unsecuredMux != nil {
			basePath := "/debug/controllers/" + controllerName
			unsecuredMux.UnlistedHandle(basePath, http.StripPrefix(basePath, debugHandler))
			unsecuredMux.UnlistedHandlePrefix(basePath+"/", http.StripPrefix(basePath, debugHandler))
		}
		klog.Infof("Started %q", controllerName)
	}

	return nil
}

// serviceAccountTokenControllerStarter is special because it must run first to set up permissions for other controllers.
// It cannot use the "normal" client builder, so it tracks its own. It must also avoid being included in the "normal"
// init map so that it can always run first.
//
// todo serviceAccountTokenControllerStarter:
//			【是特殊的，因为它必须首先运行才能为其他控制器设置权限。】
//			它不能使用“常规”客户端构建器，因此它会跟踪其自身。
//			它还必须避免包含在“常规”初始化 map 中，以便始终可以首先运行。
type serviceAccountTokenControllerStarter struct {
	rootClientBuilder controller.ControllerClientBuilder
}


// 用户帐户（User Account）和服务帐户（Service Account）
//
// 用户帐户为人提供账户标识，而服务账户为计算机进程和K8s集群中运行的Pod提供账户标识。
//
// 用户帐户和服务帐户的一个区别是作用范围；
// todo
// 		【用户帐户】对应的是人的身份，人的身份与服务的namespace无关，所以用户账户是跨namespace的；
// 		【服务帐户】对应的是一个运行中程序的身份，与特定namespace是相关的。


// 启动 ServiceAccountTokenController
//
func (c serviceAccountTokenControllerStarter) startServiceAccountTokenController(ctx ControllerContext) (http.Handler, bool, error) {

	// 先检查下 ServiceAccountTokenController 是否可以启用
	//
	// 如果 不可以，则 结束
	if !ctx.IsControllerEnabled(saTokenControllerName) {
		klog.Warningf("%q is disabled", saTokenControllerName)
		return nil, false, nil
	}

	if len(ctx.ComponentConfig.SAController.ServiceAccountKeyFile) == 0 {
		klog.Warningf("%q is disabled because there is no private key", saTokenControllerName)
		return nil, false, nil
	}
	privateKey, err := keyutil.PrivateKeyFromFile(ctx.ComponentConfig.SAController.ServiceAccountKeyFile)
	if err != nil {
		return nil, true, fmt.Errorf("error reading key for service account token controller: %v", err)
	}

	var rootCA []byte
	if ctx.ComponentConfig.SAController.RootCAFile != "" {
		if rootCA, err = readCA(ctx.ComponentConfig.SAController.RootCAFile); err != nil {
			return nil, true, fmt.Errorf("error parsing root-ca-file at %s: %v", ctx.ComponentConfig.SAController.RootCAFile, err)
		}
	} else {
		rootCA = c.rootClientBuilder.ConfigOrDie("tokens-controller").CAData
	}

	tokenGenerator, err := serviceaccount.JWTTokenGenerator(serviceaccount.LegacyIssuer, privateKey)
	if err != nil {
		return nil, false, fmt.Errorf("failed to build token generator: %v", err)
	}
	controller, err := serviceaccountcontroller.NewTokensController(
		ctx.InformerFactory.Core().V1().ServiceAccounts(),
		ctx.InformerFactory.Core().V1().Secrets(),
		c.rootClientBuilder.ClientOrDie("tokens-controller"),
		serviceaccountcontroller.TokensControllerOptions{
			TokenGenerator: tokenGenerator,
			RootCA:         rootCA,
		},
	)
	if err != nil {
		return nil, true, fmt.Errorf("error creating Tokens controller: %v", err)
	}
	go controller.Run(int(ctx.ComponentConfig.SAController.ConcurrentSATokenSyncs), ctx.Stop)

	// start the first set of informers now so that other controllers can start
	ctx.InformerFactory.Start(ctx.Stop)

	return nil, true, nil
}

func readCA(file string) ([]byte, error) {
	rootCA, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	if _, err := certutil.ParseCertsPEM(rootCA); err != nil {
		return nil, err
	}

	return rootCA, err
}

func shouldTurnOnDynamicClient(client clientset.Interface) bool {
	if !utilfeature.DefaultFeatureGate.Enabled(features.TokenRequest) {
		return false
	}
	apiResourceList, err := client.Discovery().ServerResourcesForGroupVersion(v1.SchemeGroupVersion.String())
	if err != nil {
		klog.Warningf("fetch api resource lists failed, use legacy client builder: %v", err)
		return false
	}

	for _, resource := range apiResourceList.APIResources {
		if resource.Name == "serviceaccounts/token" &&
			resource.Group == "authentication.k8s.io" &&
			sets.NewString(resource.Verbs...).Has("create") {
			return true
		}
	}

	return false
}
