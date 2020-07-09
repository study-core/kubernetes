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

package server

import (
	"os"
	"os/signal"
)

var onlyOneSignalHandler = make(chan struct{})
var shutdownHandler chan os.Signal

// SetupSignalHandler registered for SIGTERM and SIGINT. A stop channel is returned
// which is closed on one of these signals. If a second signal is caught, the program
// is terminated with exit code 1.
//
// SetupSignalHandler:
// 		已注册 `SIGTERM` 和 `SIGINT`。
//		返回一个停止通道，该通道在这些信号之一上关闭。
//		如果捕获到第二个信号，程序将以退出代码1终止。
func SetupSignalHandler() <-chan struct{} {

	// 这个通道 只是用来 做关闭用的 todo 在这里就是做 SetupSignalHandler() 只能被调用一次用的
	close(onlyOneSignalHandler) // panics when called twice  调用两次 会惊慌

	shutdownHandler = make(chan os.Signal, 2)

	stop := make(chan struct{})

	// 监听 系统信号
	signal.Notify(shutdownHandler, shutdownSignals...)
	go func() {

		// 接收到第一个信号时, 关闭 stop chan, todo 这时 监听stop chan的地方 解开阻塞
		<-shutdownHandler
		close(stop)
		// 接受到第二个信号时, exit 1 直接退出。
		<-shutdownHandler
		os.Exit(1) // second signal. Exit directly.
	}()

	return stop
}

// RequestShutdown emulates a received event that is considered as shutdown signal (SIGTERM/SIGINT)
// This returns whether a handler was notified
func RequestShutdown() bool {
	if shutdownHandler != nil {
		select {
		case shutdownHandler <- shutdownSignals[0]:
			return true
		default:
		}
	}

	return false
}
