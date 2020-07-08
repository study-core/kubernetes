/*
Copyright 2018 The Kubernetes Authors.

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

package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog"
)

// WaitForAPIServer waits for the API Server's /healthz endpoint to report "ok" with timeout.
func WaitForAPIServer(client clientset.Interface, timeout time.Duration) error {
	var lastErr error

	err := wait.PollImmediate(time.Second, timeout, func() (bool, error) {
		healthStatus := 0
		result := client.Discovery().RESTClient().Get().AbsPath("/healthz").Do(context.TODO()).StatusCode(&healthStatus)
		if result.Error() != nil {
			lastErr = fmt.Errorf("failed to get apiserver /healthz status: %v", result.Error())
			return false, nil
		}
		if healthStatus != http.StatusOK {
			content, _ := result.Raw()
			lastErr = fmt.Errorf("APIServer isn't healthy: %v", string(content))
			klog.Warningf("APIServer isn't healthy yet: %v. Waiting a little while.", string(content))
			return false, nil
		}

		return true, nil
	})

	if err != nil {
		return fmt.Errorf("%v: %v", err, lastErr)
	}

	return nil
}

// IsControllerEnabled check if a specified controller enabled or not.
//
// IsControllerEnabled 检查指定的控制器是否启用。
func IsControllerEnabled(name string, disabledByDefaultControllers sets.String, controllers []string) bool {

	// todo controllers 要 `启用` 或 `禁用`的控制器列表
	//    '*'表示 “全部由默认控制器启用”
	//    'foo' 是 “启用'foo'”
	//    '-foo'是 “禁用'foo'”
	//    特定名称的第一项获胜

	hasStar := false
	for _, ctrl := range controllers {
		if ctrl == name {
			return true
		}

		// 被禁用的
		if ctrl == "-"+name {
			return false
		}

		// 默认启动的
		if ctrl == "*" {
			hasStar = true
		}
	}
	// if we get here, there was no explicit choice
	//
	// 如果我们到达这里，就没有明确的选择
	if !hasStar {
		// nothing on by default  不做任何事
		return false
	}

	return !disabledByDefaultControllers.Has(name)
}
