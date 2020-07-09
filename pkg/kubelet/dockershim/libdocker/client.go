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

package libdocker

import (
	"time"

	dockertypes "github.com/docker/docker/api/types"
	dockercontainer "github.com/docker/docker/api/types/container"
	dockerimagetypes "github.com/docker/docker/api/types/image"
	dockerapi "github.com/docker/docker/client"
	"k8s.io/klog"
)

const (
	// https://docs.docker.com/engine/reference/api/docker_remote_api/
	// docker version should be at least 1.13.1
	MinimumDockerAPIVersion = "1.26.0"

	// Status of a container returned by ListContainers.
	StatusRunningPrefix = "Up"
	StatusCreatedPrefix = "Created"
	StatusExitedPrefix  = "Exited"

	// Fake docker endpoint
	FakeDockerEndpoint = "fake://"
)

// Interface is an abstract interface for testability.  It abstracts the interface of docker client.
//
// 接口是可测试性的抽象接口。 它抽象了Docker客户端的接口
type Interface interface {
	ListContainers(options dockertypes.ContainerListOptions) ([]dockertypes.Container, error)
	InspectContainer(id string) (*dockertypes.ContainerJSON, error)
	InspectContainerWithSize(id string) (*dockertypes.ContainerJSON, error)
	CreateContainer(dockertypes.ContainerCreateConfig) (*dockercontainer.ContainerCreateCreatedBody, error)
	StartContainer(id string) error
	StopContainer(id string, timeout time.Duration) error
	UpdateContainerResources(id string, updateConfig dockercontainer.UpdateConfig) error
	RemoveContainer(id string, opts dockertypes.ContainerRemoveOptions) error
	InspectImageByRef(imageRef string) (*dockertypes.ImageInspect, error)
	InspectImageByID(imageID string) (*dockertypes.ImageInspect, error)
	ListImages(opts dockertypes.ImageListOptions) ([]dockertypes.ImageSummary, error)
	PullImage(image string, auth dockertypes.AuthConfig, opts dockertypes.ImagePullOptions) error
	RemoveImage(image string, opts dockertypes.ImageRemoveOptions) ([]dockertypes.ImageDeleteResponseItem, error)
	ImageHistory(id string) ([]dockerimagetypes.HistoryResponseItem, error)
	Logs(string, dockertypes.ContainerLogsOptions, StreamOptions) error
	Version() (*dockertypes.Version, error)
	Info() (*dockertypes.Info, error)
	CreateExec(string, dockertypes.ExecConfig) (*dockertypes.IDResponse, error)
	StartExec(string, dockertypes.ExecStartCheck, StreamOptions) error
	InspectExec(id string) (*dockertypes.ContainerExecInspect, error)
	AttachToContainer(string, dockertypes.ContainerAttachOptions, StreamOptions) error
	ResizeContainerTTY(id string, height, width uint) error
	ResizeExecTTY(id string, height, width uint) error
	GetContainerStats(id string) (*dockertypes.StatsJSON, error)
}

// Get a *dockerapi.Client, either using the endpoint passed in, or using
// DOCKER_HOST, DOCKER_TLS_VERIFY, and DOCKER_CERT path per their spec
//
// 根据传入的 endpoint 或使用其规范的`DOCKER_HOST`, `DOCKER_TLS_VERIFY`, 和 `DOCKER_CERT` 路径获取一个* dockerapi.Client
func getDockerClient(dockerEndpoint string) (*dockerapi.Client, error) {
	if len(dockerEndpoint) > 0 {
		klog.Infof("Connecting to docker on %s", dockerEndpoint)

		// NewClient为给定的主机和API版本初始化一个新的API客户端。
		//它使用给定的http客户端作为传输。
		//它还初始化要添加到每个请求的自定义http标头。
		//
		//如果版本号为空，则不会发送任何版本信息。 强烈建议您设置版本，否则如果升级服务器，客户端可能会损坏。
		//不推荐使用：使用NewClientWithOpts
		//
		// todo 这里 直接调用 Docker 的 方法
		return dockerapi.NewClient(dockerEndpoint, "", nil, nil)
	}

	// 使用默认的配置
	return dockerapi.NewClientWithOpts(dockerapi.FromEnv)
}

// ConnectToDockerOrDie creates docker client connecting to docker daemon.
// If the endpoint passed in is "fake://", a fake docker client
// will be returned. The program exits if error occurs. The requestTimeout
// is the timeout for docker requests. If timeout is exceeded, the request
// will be cancelled and throw out an error. If requestTimeout is 0, a default
// value will be applied.
//
//
// ConnectToDockerOrDie:
//		创建连接到Docker守护程序的Docker客户端。
//
//	如果传入的端点是 "fake：//", 则将返回伪造的Docker客户端.
//	如果发生错误, 程序将退出.
//	requestTimeout是docker请求的超时.
//	如果超过了超时, 该请求将被取消并抛出错误.
//	如果requestTimeout为0, 则将应用默认值.
func ConnectToDockerOrDie(dockerEndpoint string, requestTimeout, imagePullProgressDeadline time.Duration) Interface {

	// 根据 dockerEndpoint 记录的路径加载一个 docker 客户端
	client, err := getDockerClient(dockerEndpoint)
	if err != nil {
		klog.Fatalf("Couldn't connect to docker: %v", err)
	}
	klog.Infof("Start docker client with request timeout=%v", requestTimeout)

	// 根据 Docker 客户端 实例化一个 k8s的docker客户端
	return newKubeDockerClient(client, requestTimeout, imagePullProgressDeadline)
}
