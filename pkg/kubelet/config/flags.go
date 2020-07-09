/*
Copyright 2017 The Kubernetes Authors.

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

package config

import (
	"fmt"

	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ContainerRuntimeOptions defines options for the container runtime.
//
// ContainerRuntimeOptions定义容器运行时的选项
type ContainerRuntimeOptions struct {
	// General Options.
	//
	// 常规选项。



	// ContainerRuntime is the container runtime to use.
	//
	// ContainerRuntime 是要使用的容器运行时
	ContainerRuntime string

	// RuntimeCgroups that container runtime is expected to be isolated in.
	//
	// 希望 隔离容器运行时的 RuntimeCgroups
	RuntimeCgroups string


	// RedirectContainerStreaming enables container streaming redirect.
	// When RedirectContainerStreaming is false, kubelet will proxy container streaming data
	// between apiserver and container runtime. This approach is more secure, but the proxy
	// introduces some overhead.
	// When RedirectContainerStreaming is true, kubelet will return an http redirect to apiserver,
	// and apiserver will access container runtime directly. This approach is more performant,
	// but less secure because the connection between apiserver and container runtime is not
	// authenticated.
	//
	// todo RedirectContainerStreaming 启用容器流重定向。
	//
	// todo 如果RedirectContainerStreaming为false，
	//		kubelet将在apiserver和 container 运行时之间 代理容container流数据。 这种方法更安全，但是代理会带来一些开销。
	//
	// todo 如果RedirectContainerStreaming为true，
	// 		kubelet将向apiserver返回一个http重定向，并且apiserver将直接访问container runtime。
	//		由于未验证apiserver和 container运行时之间的连接，因此该方法性能更高，但安全性更低。
	RedirectContainerStreaming bool


	// Docker-specific options.
	//
	// 特定于Docker的选项

	// DockershimRootDirectory is the path to the dockershim root directory. Defaults to
	// /var/lib/dockershim if unset. Exposed for integration testing (e.g. in OpenShift).
	//
	// DockershimRootDirectory是dockershim根目录的路径。
	// 如果未设置，默认为`/ var / lib / dockershim`。
	// 公开进行集成测试（例如在OpenShift中）。
	DockershimRootDirectory string


	// Enable dockershim only mode.
	//
	// 启用仅dockershim模式。
	ExperimentalDockershim bool


	// PodSandboxImage is the image whose network/ipc namespaces
	// containers in each pod will use.
	//
	// PodSandboxImage: 是每个Pod中将使用其 网络/ ipc名称空间 容器镜像 (container image)
	PodSandboxImage string


	// DockerEndpoint is the path to the docker endpoint to communicate with.
	//
	// DockerEndpoint: 是与当前 节点<kubelet>实例 通信的Docker端点的路径
	DockerEndpoint string


	// If no pulling progress is made before the deadline imagePullProgressDeadline,
	// the image pulling will be cancelled. Defaults to 1m0s.
	// +optional
	//
	// 如果在截止时间 `imagePullProgressDeadline` 之前没有进行拉动进度，则将取消 镜像拉动。 默认为1m0s。 +可选
	ImagePullProgressDeadline metav1.Duration

	// Network plugin options.
	//
	// 网络插件选项

	// networkPluginName is the name of the network plugin to be invoked for
	// various events in kubelet/pod lifecycle
	//
	// networkPluginName 是在kubelet / pod生命周期中为各种事件调用的网络插件的名称
	NetworkPluginName string


	// NetworkPluginMTU is the MTU to be passed to the network plugin,
	// and overrides the default MTU for cases where it cannot be automatically
	// computed (such as IPSEC).
	//
	// NetworkPluginMTU: 是要传递给网络插件的MTU，对于无法自动计算的情况（例如IPSEC），它会覆盖默认MTU。
	NetworkPluginMTU int32


	// CNIConfDir is the full path of the directory in which to search for
	// CNI config files
	//
	// CNIConfDir: 是要在其中搜索CNI配置文件的目录的完整路径
	CNIConfDir string


	// CNIBinDir is the full path of the directory in which to search for
	// CNI plugin binaries
	//
	// CNIBinDir: 是要在其中搜索CNI插件二进制文件的目录的完整路径
	CNIBinDir string


	// CNICacheDir is the full path of the directory in which CNI should store
	// cache files
	//
	// CNICacheDir: 是CNI应在其中存储缓存文件的目录的完整路径
	CNICacheDir string
}

// AddFlags adds flags to the container runtime, according to ContainerRuntimeOptions.
func (s *ContainerRuntimeOptions) AddFlags(fs *pflag.FlagSet) {
	dockerOnlyWarning := "This docker-specific flag only works when container-runtime is set to docker."

	// General settings.
	fs.StringVar(&s.ContainerRuntime, "container-runtime", s.ContainerRuntime, "The container runtime to use. Possible values: 'docker', 'remote'.")
	fs.StringVar(&s.RuntimeCgroups, "runtime-cgroups", s.RuntimeCgroups, "Optional absolute name of cgroups to create and run the runtime in.")
	fs.BoolVar(&s.RedirectContainerStreaming, "redirect-container-streaming", s.RedirectContainerStreaming, "Enables container streaming redirect. If false, kubelet will proxy container streaming data between apiserver and container runtime; if true, kubelet will return an http redirect to apiserver, and apiserver will access container runtime directly. The proxy approach is more secure, but introduces some overhead. The redirect approach is more performant, but less secure because the connection between apiserver and container runtime may not be authenticated.")
	fs.MarkDeprecated("redirect-container-streaming", "Container streaming redirection will be removed from the kubelet in v1.20, and this flag will be removed in v1.22. For more details, see http://git.k8s.io/enhancements/keps/sig-node/20191205-container-streaming-requests.md")

	// Docker-specific settings.
	fs.BoolVar(&s.ExperimentalDockershim, "experimental-dockershim", s.ExperimentalDockershim, "Enable dockershim only mode. In this mode, kubelet will only start dockershim without any other functionalities. This flag only serves test purpose, please do not use it unless you are conscious of what you are doing. [default=false]")
	fs.MarkHidden("experimental-dockershim")
	fs.StringVar(&s.DockershimRootDirectory, "experimental-dockershim-root-directory", s.DockershimRootDirectory, "Path to the dockershim root directory.")
	fs.MarkHidden("experimental-dockershim-root-directory")
	fs.StringVar(&s.PodSandboxImage, "pod-infra-container-image", s.PodSandboxImage, fmt.Sprintf("The image whose network/ipc namespaces containers in each pod will use. %s", dockerOnlyWarning))
	fs.StringVar(&s.DockerEndpoint, "docker-endpoint", s.DockerEndpoint, fmt.Sprintf("Use this for the docker endpoint to communicate with. %s", dockerOnlyWarning))
	fs.DurationVar(&s.ImagePullProgressDeadline.Duration, "image-pull-progress-deadline", s.ImagePullProgressDeadline.Duration, fmt.Sprintf("If no pulling progress is made before this deadline, the image pulling will be cancelled. %s", dockerOnlyWarning))

	// Network plugin settings for Docker.
	fs.StringVar(&s.NetworkPluginName, "network-plugin", s.NetworkPluginName, fmt.Sprintf("<Warning: Alpha feature> The name of the network plugin to be invoked for various events in kubelet/pod lifecycle. %s", dockerOnlyWarning))
	fs.StringVar(&s.CNIConfDir, "cni-conf-dir", s.CNIConfDir, fmt.Sprintf("<Warning: Alpha feature> The full path of the directory in which to search for CNI config files. %s", dockerOnlyWarning))
	fs.StringVar(&s.CNIBinDir, "cni-bin-dir", s.CNIBinDir, fmt.Sprintf("<Warning: Alpha feature> A comma-separated list of full paths of directories in which to search for CNI plugin binaries. %s", dockerOnlyWarning))
	fs.StringVar(&s.CNICacheDir, "cni-cache-dir", s.CNICacheDir, fmt.Sprintf("<Warning: Alpha feature> The full path of the directory in which CNI should store cache files. %s", dockerOnlyWarning))
	fs.Int32Var(&s.NetworkPluginMTU, "network-plugin-mtu", s.NetworkPluginMTU, fmt.Sprintf("<Warning: Alpha feature> The MTU to be passed to the network plugin, to override the default. Set to 0 to use the default 1460 MTU. %s", dockerOnlyWarning))
}
