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

// The kubelet binary is responsible for maintaining a set of containers on a particular host VM.
// It syncs data from both configuration file(s) as well as from a quorum of etcd servers.
// It then queries Docker to see what is currently running.  It synchronizes the configuration data,
// with the running set of containers by starting or stopping Docker containers.
package main

import (
	"math/rand"
	"os"
	"time"

	"k8s.io/component-base/logs"
	_ "k8s.io/component-base/metrics/prometheus/restclient"
	_ "k8s.io/component-base/metrics/prometheus/version" // for version metric registration
	"k8s.io/kubernetes/cmd/kubelet/app"
)

// 每个Node节点上都运行一个 Kubelet 服务进程，
// 默认监听 10250 端口，接收并执行 Master 发来的指令，
// 管理 Pod 及 Pod 中的容器.
//
// 每个 Kubelet 进程会在 API Server 上注册所在Node节点的信息，
// 定期向 Master 节点汇报该节点的资源使用情况，并通过 cAdvisor 监控节点和容器的资源.

// todo kubelet 组件的入口
func main() {

	// 设置全局的随机种子
	rand.Seed(time.Now().UnixNano())


	// 初始化 kubelet cmd 实例
	command := app.NewKubeletCommand()
	logs.InitLogs()
	defer logs.FlushLogs()

	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}
