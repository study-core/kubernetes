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

package options

import (
	"github.com/spf13/pflag"

	persistentvolumeconfig "k8s.io/kubernetes/pkg/controller/volume/persistentvolume/config"
)

// PersistentVolumeBinderControllerOptions holds the PersistentVolumeBinderController options.
type PersistentVolumeBinderControllerOptions struct {
	*persistentvolumeconfig.PersistentVolumeBinderControllerConfiguration
}

// AddFlags adds flags related to PersistentVolumeBinderController for controller manager to the specified FlagSet.
//
// AddFlags:
//     将与用于 ControllerManager 的 PersistentVolumeBinderController [连续卷绑定控制选项] 相关的标志添加到指定的FlagSet。
func (o *PersistentVolumeBinderControllerOptions) AddFlags(fs *pflag.FlagSet) {
	if o == nil {
		return
	}

	// todo 持久存储卷（Persistent Volume，PV）和 持久存储卷声明（Persistent Volume Claim，PVC）
	//
	// todo PV和PVC使得K8s集群具备了存储的逻辑抽象能力，
	//      使得在配置Pod的逻辑里可以忽略对实际后台存储技术的配置，
	//      而把这项配置的工作交给PV的配置者，即集群的管理者。
	//
	// todo 存储的PV和PVC的这种关系，跟计算的Node和Pod的关系是非常类似的；
	//      PV和Node是资源的提供者，根据集群的基础设施变化而变化，由K8s集群管理员配置；
	//      而PVC和Pod是资源的使用者，根据业务服务的需求变化而变化，由K8s集群的使用者即服务的管理员来配置。

	fs.DurationVar(&o.PVClaimBinderSyncPeriod.Duration, "pvclaimbinder-sync-period", o.PVClaimBinderSyncPeriod.Duration, "The period for syncing persistent volumes and persistent volume claims")
	fs.StringVar(&o.VolumeConfiguration.PersistentVolumeRecyclerConfiguration.PodTemplateFilePathNFS, "pv-recycler-pod-template-filepath-nfs", o.VolumeConfiguration.PersistentVolumeRecyclerConfiguration.PodTemplateFilePathNFS, "The file path to a pod definition used as a template for NFS persistent volume recycling")
	fs.Int32Var(&o.VolumeConfiguration.PersistentVolumeRecyclerConfiguration.MinimumTimeoutNFS, "pv-recycler-minimum-timeout-nfs", o.VolumeConfiguration.PersistentVolumeRecyclerConfiguration.MinimumTimeoutNFS, "The minimum ActiveDeadlineSeconds to use for an NFS Recycler pod")
	fs.Int32Var(&o.VolumeConfiguration.PersistentVolumeRecyclerConfiguration.IncrementTimeoutNFS, "pv-recycler-increment-timeout-nfs", o.VolumeConfiguration.PersistentVolumeRecyclerConfiguration.IncrementTimeoutNFS, "the increment of time added per Gi to ActiveDeadlineSeconds for an NFS scrubber pod")
	fs.StringVar(&o.VolumeConfiguration.PersistentVolumeRecyclerConfiguration.PodTemplateFilePathHostPath, "pv-recycler-pod-template-filepath-hostpath", o.VolumeConfiguration.PersistentVolumeRecyclerConfiguration.PodTemplateFilePathHostPath, "The file path to a pod definition used as a template for HostPath persistent volume recycling. This is for development and testing only and will not work in a multi-node cluster.")
	fs.Int32Var(&o.VolumeConfiguration.PersistentVolumeRecyclerConfiguration.MinimumTimeoutHostPath, "pv-recycler-minimum-timeout-hostpath", o.VolumeConfiguration.PersistentVolumeRecyclerConfiguration.MinimumTimeoutHostPath, "The minimum ActiveDeadlineSeconds to use for a HostPath Recycler pod.  This is for development and testing only and will not work in a multi-node cluster.")
	fs.Int32Var(&o.VolumeConfiguration.PersistentVolumeRecyclerConfiguration.IncrementTimeoutHostPath, "pv-recycler-timeout-increment-hostpath", o.VolumeConfiguration.PersistentVolumeRecyclerConfiguration.IncrementTimeoutHostPath, "the increment of time added per Gi to ActiveDeadlineSeconds for a HostPath scrubber pod.  This is for development and testing only and will not work in a multi-node cluster.")
	fs.BoolVar(&o.VolumeConfiguration.EnableHostPathProvisioning, "enable-hostpath-provisioner", o.VolumeConfiguration.EnableHostPathProvisioning, "Enable HostPath PV provisioning when running without a cloud provider. This allows testing and development of provisioning features.  HostPath provisioning is not supported in any way, won't work in a multi-node cluster, and should not be used for anything other than testing or development.")
	fs.BoolVar(&o.VolumeConfiguration.EnableDynamicProvisioning, "enable-dynamic-provisioning", o.VolumeConfiguration.EnableDynamicProvisioning, "Enable dynamic provisioning for environments that support it.")
	fs.StringVar(&o.VolumeConfiguration.FlexVolumePluginDir, "flex-volume-plugin-dir", o.VolumeConfiguration.FlexVolumePluginDir, "Full path of the directory in which the flex volume plugin should search for additional third party volume plugins.")
}

// ApplyTo fills up PersistentVolumeBinderController config with options.
func (o *PersistentVolumeBinderControllerOptions) ApplyTo(cfg *persistentvolumeconfig.PersistentVolumeBinderControllerConfiguration) error {
	if o == nil {
		return nil
	}

	cfg.PVClaimBinderSyncPeriod = o.PVClaimBinderSyncPeriod
	cfg.VolumeConfiguration = o.VolumeConfiguration

	return nil
}

// Validate checks validation of PersistentVolumeBinderControllerOptions.
func (o *PersistentVolumeBinderControllerOptions) Validate() []error {
	if o == nil {
		return nil
	}

	errs := []error{}
	return errs
}
