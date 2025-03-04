// Copyright 2019 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package validation

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	"github.com/pingcap/tidb-operator/pkg/apis/label"
	"github.com/pingcap/tidb-operator/pkg/apis/pingcap/v1alpha1"
	"github.com/prometheus/common/model"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	utilnet "k8s.io/utils/net"
)

// ValidateTidbCluster validates a TidbCluster, it performs basic validation for all TidbClusters despite it is legacy
// or not
func ValidateTidbCluster(tc *v1alpha1.TidbCluster) field.ErrorList {
	allErrs := field.ErrorList{}
	// validate metadata
	fldPath := field.NewPath("metadata")
	// validate metadata/annotations
	allErrs = append(allErrs, validateAnnotations(tc.ObjectMeta.Annotations, fldPath.Child("annotations"))...)
	// validate spec
	allErrs = append(allErrs, validateTiDBClusterSpec(&tc.Spec, field.NewPath("spec"))...)
	return allErrs
}

// ValidateDMCluster validates a DMCluster, it performs basic validation for all DMClusters despite it is legacy
// or not
func ValidateDMCluster(dc *v1alpha1.DMCluster) field.ErrorList {
	allErrs := field.ErrorList{}
	// validate metadata
	fldPath := field.NewPath("metadata")
	// validate metadata/annotations
	allErrs = append(allErrs, validateDMAnnotations(dc.ObjectMeta.Annotations, fldPath.Child("annotations"))...)
	// validate spec
	allErrs = append(allErrs, validateDMClusterSpec(&dc.Spec, field.NewPath("spec"))...)
	return allErrs
}

// ValidateTiDBNGMonitoring validates a TidbNGMonitoring
func ValidateTiDBNGMonitoring(tngm *v1alpha1.TidbNGMonitoring) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateTidbNGMonitorinSpec(&tngm.Spec, field.NewPath("spec"))...)

	return allErrs
}

func ValidateTidbMonitor(monitor *v1alpha1.TidbMonitor) field.ErrorList {
	allErrs := field.ErrorList{}
	// validate monitor service
	if monitor.Spec.Grafana != nil {
		allErrs = append(allErrs, validateService(&monitor.Spec.Grafana.Service, field.NewPath("spec"))...)
	}

	allErrs = append(allErrs, validateService(&monitor.Spec.Prometheus.Service, field.NewPath("spec"))...)
	allErrs = append(allErrs, validatePromDurationStr(monitor.Spec.Prometheus.RetentionTime, field.NewPath("spec"))...)
	allErrs = append(allErrs, validateService(&monitor.Spec.Reloader.Service, field.NewPath("spec"))...)
	if monitor.Spec.Persistent {
		allErrs = append(allErrs, validateStorageInfo(monitor.Spec.Storage, field.NewPath("spec"))...)
	}
	return allErrs
}

func validateAnnotations(anns map[string]string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateAnnotations(anns, fldPath)...)
	for _, key := range []string{label.AnnPDDeleteSlots, label.AnnTiDBDeleteSlots, label.AnnTiKVDeleteSlots, label.AnnTiFlashDeleteSlots} {
		allErrs = append(allErrs, validateDeleteSlots(anns, key, fldPath.Child(key))...)
	}
	return allErrs
}

func validateDMAnnotations(anns map[string]string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateAnnotations(anns, fldPath)...)
	for _, key := range []string{label.AnnDMMasterDeleteSlots, label.AnnDMWorkerDeleteSlots} {
		allErrs = append(allErrs, validateDeleteSlots(anns, key, fldPath.Child(key))...)
	}
	return allErrs
}

func validateTiDBClusterSpec(spec *v1alpha1.TidbClusterSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateDiscoverySpec(spec.Discovery, fldPath.Child("discovery"))...)
	if spec.PD != nil {
		allErrs = append(allErrs, validatePDSpec(spec.PD, fldPath.Child("pd"))...)
	}
	if spec.TiKV != nil {
		allErrs = append(allErrs, validateTiKVSpec(spec.TiKV, fldPath.Child("tikv"))...)
	}
	if spec.TiDB != nil {
		allErrs = append(allErrs, validateTiDBSpec(spec.TiDB, fldPath.Child("tidb"))...)
	}
	if spec.Pump != nil {
		allErrs = append(allErrs, validatePumpSpec(spec.Pump, fldPath.Child("pump"))...)
	}
	if spec.TiFlash != nil {
		allErrs = append(allErrs, validateTiFlashSpec(spec.TiFlash, fldPath.Child("tiflash"))...)
	}
	if spec.TiCDC != nil {
		allErrs = append(allErrs, validateTiCDCSpec(spec.TiCDC, fldPath.Child("ticdc"))...)
	}
	if spec.PDAddresses != nil {
		allErrs = append(allErrs, validatePDAddresses(spec.PDAddresses, fldPath.Child("pdAddresses"))...)
	}
	return allErrs
}

func validateDiscoverySpec(spec v1alpha1.DiscoverySpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if spec.ComponentSpec != nil {
		allErrs = append(allErrs, validateComponentSpec(spec.ComponentSpec, fldPath)...)
	}
	return allErrs
}

func validatePDSpec(spec *v1alpha1.PDSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validateComponentSpec(&spec.ComponentSpec, fldPath)...)
	allErrs = append(allErrs, validateRequestsStorage(spec.ResourceRequirements.Requests, fldPath)...)
	if len(spec.StorageVolumes) > 0 {
		allErrs = append(allErrs, validateStorageVolumes(spec.StorageVolumes, fldPath.Child("storageVolumes"))...)
	}
	if spec.Service != nil {
		allErrs = append(allErrs, validateService(spec.Service, fldPath)...)
	}
	return allErrs
}

func validatePDAddresses(arrayOfAddresses []string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	for i, address := range arrayOfAddresses {
		idxPath := fldPath.Index(i)
		u, err := url.Parse(address)
		example := " PD address format example: http://{ADDRESS}:{PORT}"
		if err != nil {
			allErrs = append(allErrs, field.Invalid(idxPath, address, err.Error()+example))
		} else if u.Scheme != "http" {
			allErrs = append(allErrs, field.Invalid(idxPath, address, "Support 'http' scheme only."+example))
		}
	}
	return allErrs
}

func validateTiKVSpec(spec *v1alpha1.TiKVSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validateComponentSpec(&spec.ComponentSpec, fldPath)...)
	allErrs = append(allErrs, validateRequestsStorage(spec.ResourceRequirements.Requests, fldPath)...)
	if len(spec.DataSubDir) > 0 {
		allErrs = append(allErrs, validateLocalDescendingPath(spec.DataSubDir, fldPath.Child("dataSubDir"))...)
	}
	if len(spec.StorageVolumes) > 0 {
		allErrs = append(allErrs, validateStorageVolumes(spec.StorageVolumes, fldPath.Child("storageVolumes"))...)
	}
	if spec.ShouldSeparateRaftLog() && spec.RaftLogVolumeName != "" {
		allErrs = append(allErrs, validateVolumeName(spec.RaftLogVolumeName, spec.StorageVolumes, spec.AdditionalVolumes, spec.AdditionalVolumeMounts, fldPath)...)
	}
	if spec.ShouldSeparateRocksDBLog() && spec.RocksDBLogVolumeName != "" {
		allErrs = append(allErrs, validateVolumeName(spec.RocksDBLogVolumeName, spec.StorageVolumes, spec.AdditionalVolumes, spec.AdditionalVolumeMounts, fldPath)...)
	}
	allErrs = append(allErrs, validateTimeDurationStr(spec.EvictLeaderTimeout, fldPath.Child("evictLeaderTimeout"))...)
	return allErrs
}

func validateTiFlashSpec(spec *v1alpha1.TiFlashSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validateComponentSpec(&spec.ComponentSpec, fldPath)...)
	allErrs = append(allErrs, validateTiFlashConfig(spec.Config, fldPath)...)
	if len(spec.StorageClaims) < 1 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("spec.StorageClaims"),
			spec.StorageClaims, "storageClaims should be configured at least one item."))
	}
	return allErrs
}

func validateTiCDCSpec(spec *v1alpha1.TiCDCSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validateComponentSpec(&spec.ComponentSpec, fldPath)...)
	if len(spec.StorageVolumes) > 0 {
		allErrs = append(allErrs, validateStorageVolumes(spec.StorageVolumes, fldPath.Child("storageVolumes"))...)
	}
	return allErrs
}

func validateTiFlashConfig(config *v1alpha1.TiFlashConfigWraper, path *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if config == nil {
		return allErrs
	}

	if config.Common != nil {
		if v := config.Common.Get("flash.overlap_threshold"); v != nil {
			if value, err := v.AsFloat(); err == nil {
				if value < 0 || value > 1 {
					allErrs = append(allErrs, field.Invalid(path.Child("config.config.flash.overlap_threshold"),
						value,
						"overlap_threshold must be in the range of [0,1]."))
				}
			} else {
				allErrs = append(allErrs, field.Invalid(path.Child("config.config.flash.overlap_threshold"),
					v.Interface(),
					fmt.Sprintf("should be float type, but is: %v", reflect.TypeOf(v.Interface())),
				))
			}
		}

		var fields = []string{
			"flash.flash_cluster.log",
			"flash.proxy.log-file",
			"logger.log",
			"logger.errorlog",
		}
		for _, pathField := range fields {
			if v := config.Common.Get(pathField); v != nil {
				if value, err := v.AsString(); err == nil {
					splitPath := strings.Split(value, string(os.PathSeparator))
					// The log path should be at least /dir/base.log
					if len(splitPath) < 3 {
						allErrs = append(allErrs, field.Invalid(path.Child("config.config."+pathField),
							value,
							"log path should include at least one level dir."))
					}
				} else {
					allErrs = append(allErrs, field.Invalid(path.Child("config.config"+pathField),
						v.Interface(),
						fmt.Sprintf("should be string type, but is: %v", reflect.TypeOf(v.Interface())),
					))
				}
			}
		}
	}

	return allErrs
}

func validateTiDBSpec(spec *v1alpha1.TiDBSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validateComponentSpec(&spec.ComponentSpec, fldPath)...)
	if spec.Service != nil {
		allErrs = append(allErrs, validateService(&spec.Service.ServiceSpec, fldPath)...)
	}
	if len(spec.StorageVolumes) > 0 {
		allErrs = append(allErrs, validateStorageVolumes(spec.StorageVolumes, fldPath.Child("storageVolumes"))...)
	}
	if spec.ShouldSeparateSlowLog() && spec.SlowLogVolumeName != "" {
		allErrs = append(allErrs, validateVolumeName(spec.SlowLogVolumeName, spec.StorageVolumes, spec.AdditionalVolumes, spec.AdditionalVolumeMounts, fldPath)...)
	}
	return allErrs
}

func validatePumpSpec(spec *v1alpha1.PumpSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validateComponentSpec(&spec.ComponentSpec, fldPath)...)
	// fix pump spec
	if _, ok := spec.ResourceRequirements.Requests["storage"]; !ok {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("spec.ResourceRequirements.Requests"),
			spec.ResourceRequirements.Requests, "spec.ResourceRequirements.Requests[storage]: Required value."))
	}
	return allErrs
}

func validateDMClusterSpec(spec *v1alpha1.DMClusterSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if spec.Version != "" {
		clusterVersionLT2, _ := clusterVersionLessThan2(spec.Version)
		if clusterVersionLT2 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("version"), spec.Version, "dm cluster version can't set to v1.x.y"))
		}
	}
	allErrs = append(allErrs, validateDMDiscoverySpec(spec.Discovery, fldPath.Child("discovery"))...)
	allErrs = append(allErrs, validateMasterSpec(&spec.Master, fldPath.Child("master"))...)
	if spec.Worker != nil {
		allErrs = append(allErrs, validateWorkerSpec(spec.Worker, fldPath.Child("worker"))...)
	}
	return allErrs
}

func validateDMDiscoverySpec(spec v1alpha1.DMDiscoverySpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if spec.ComponentSpec != nil {
		allErrs = append(allErrs, validateComponentSpec(spec.ComponentSpec, fldPath)...)
	}
	return allErrs
}

func validateMasterSpec(spec *v1alpha1.MasterSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validateComponentSpec(&spec.ComponentSpec, fldPath)...)
	// make sure that storageSize for dm-master is assigned
	if spec.Replicas > 0 && spec.StorageSize == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("storageSize"), "storageSize must not be empty"))
	}
	return allErrs
}

func validateWorkerSpec(spec *v1alpha1.WorkerSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validateComponentSpec(&spec.ComponentSpec, fldPath)...)
	return allErrs
}

func validateTidbNGMonitorinSpec(spec *v1alpha1.TidbNGMonitoringSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(spec.Clusters) < 1 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("clusters"), len(spec.Clusters), "must have at least one item"))
	}
	allErrs = append(allErrs, validateComponentSpec(&spec.ComponentSpec, fldPath)...)
	allErrs = append(allErrs, validateNGMonitoringSpec(&spec.NGMonitoring, fldPath.Child("ngMonitoring"))...)

	return allErrs
}

func validateNGMonitoringSpec(spec *v1alpha1.NGMonitoringSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateComponentSpec(&spec.ComponentSpec, fldPath)...)
	if len(spec.StorageVolumes) > 0 {
		allErrs = append(allErrs, validateStorageVolumes(spec.StorageVolumes, fldPath.Child("storageVolumes"))...)
	}

	return allErrs
}

func validateComponentSpec(spec *v1alpha1.ComponentSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	// TODO validate other fields
	allErrs = append(allErrs, validateEnv(spec.Env, fldPath.Child("env"))...)
	allErrs = append(allErrs, validateAdditionalContainers(spec.AdditionalContainers, fldPath.Child("additionalContainers"))...)
	return allErrs
}

//validateRequestsStorage validates resources requests storage
func validateRequestsStorage(requests corev1.ResourceList, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if _, ok := requests[corev1.ResourceStorage]; !ok {
		allErrs = append(allErrs, field.Required(fldPath.Child("requests.storage").Key((string(corev1.ResourceStorage))), "storage request must not be empty"))
	}
	return allErrs
}

//validateTiKVStorageSize validates resources requests storage
func validateStorageVolumes(storageVolumes []v1alpha1.StorageVolume, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	for i, storageVolume := range storageVolumes {
		idxPath := fldPath.Index(i)
		if len(storageVolume.Name) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("name"), "name must not be empty"))
		}
		_, err := resource.ParseQuantity(storageVolume.StorageSize)
		if err != nil {
			allErrs = append(allErrs, &field.Error{
				Type:   field.ErrorTypeNotSupported,
				Detail: `value of "storageSize" format not supported`,
			})
		}
	}
	return allErrs
}

func validateVolumeName(volumeName string, storageVolumes []v1alpha1.StorageVolume, additionalVolumes []corev1.Volume, additionalVolumeMounts []corev1.VolumeMount, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	for _, volume := range storageVolumes {
		if volume.Name == volumeName {
			return allErrs
		}
	}
	for _, volume := range additionalVolumes {
		if volume.Name == volumeName {
			for _, volumeMount := range additionalVolumeMounts {
				if volumeMount.Name == volumeName {
					return allErrs
				}
			}
		}
	}
	errMsg := fmt.Sprintf("Can not find volumeName: %s in storageVolumes or additionalVolumes/additionalVolumeMounts", volumeName)
	allErrs = append(allErrs, field.Invalid(fldPath.Child("volumeName"), volumeName, errMsg))
	return allErrs
}

// validateEnv validates env vars
func validateEnv(vars []corev1.EnvVar, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for i, ev := range vars {
		idxPath := fldPath.Index(i)
		if len(ev.Name) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("name"), ""))
		} else {
			for _, msg := range validation.IsEnvVarName(ev.Name) {
				allErrs = append(allErrs, field.Invalid(idxPath.Child("name"), ev.Name, msg))
			}
		}
		allErrs = append(allErrs, validateEnvVarValueFrom(ev, idxPath.Child("valueFrom"))...)
	}
	return allErrs
}

func validateEnvVarValueFrom(ev corev1.EnvVar, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if ev.ValueFrom == nil {
		return allErrs
	}

	numSources := 0

	if ev.ValueFrom.FieldRef != nil {
		numSources++
		allErrs = append(allErrs, field.Invalid(fldPath.Child("fieldRef"), "", "fieldRef is not supported"))
	}
	if ev.ValueFrom.ResourceFieldRef != nil {
		numSources++
		allErrs = append(allErrs, field.Invalid(fldPath.Child("resourceFieldRef"), "", "resourceFieldRef is not supported"))
	}
	if ev.ValueFrom.ConfigMapKeyRef != nil {
		numSources++
		allErrs = append(allErrs, validateConfigMapKeySelector(ev.ValueFrom.ConfigMapKeyRef, fldPath.Child("configMapKeyRef"))...)
	}
	if ev.ValueFrom.SecretKeyRef != nil {
		numSources++
		allErrs = append(allErrs, validateSecretKeySelector(ev.ValueFrom.SecretKeyRef, fldPath.Child("secretKeyRef"))...)
	}

	if numSources == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, "", "must specify one of: `configMapKeyRef` or `secretKeyRef`"))
	} else if len(ev.Value) != 0 {
		if numSources != 0 {
			allErrs = append(allErrs, field.Invalid(fldPath, "", "may not be specified when `value` is not empty"))
		}
	} else if numSources > 1 {
		allErrs = append(allErrs, field.Invalid(fldPath, "", "may not have more than one field specified at a time"))
	}

	return allErrs
}

func validateConfigMapKeySelector(s *corev1.ConfigMapKeySelector, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for _, msg := range apivalidation.NameIsDNSSubdomain(s.Name, false) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), s.Name, msg))
	}
	if len(s.Key) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("key"), ""))
	} else {
		for _, msg := range validation.IsConfigMapKey(s.Key) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("key"), s.Key, msg))
		}
	}

	return allErrs
}

func validateSecretKeySelector(s *corev1.SecretKeySelector, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for _, msg := range apivalidation.NameIsDNSSubdomain(s.Name, false) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), s.Name, msg))
	}
	if len(s.Key) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("key"), ""))
	} else {
		for _, msg := range validation.IsConfigMapKey(s.Key) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("key"), s.Key, msg))
		}
	}

	return allErrs
}

// ValidateCreateTidbCLuster validates a newly created TidbCluster
func ValidateCreateTidbCluster(tc *v1alpha1.TidbCluster) field.ErrorList {
	allErrs := field.ErrorList{}
	// basic validation
	allErrs = append(allErrs, ValidateTidbCluster(tc)...)
	allErrs = append(allErrs, validateNewTidbClusterSpec(&tc.Spec, field.NewPath("spec"))...)
	return allErrs
}

// ValidateUpdateTidbCluster validates a new TidbCluster against an existing TidbCluster to be updated
func ValidateUpdateTidbCluster(old, tc *v1alpha1.TidbCluster) field.ErrorList {

	allErrs := field.ErrorList{}
	// basic validation
	allErrs = append(allErrs, ValidateTidbCluster(tc)...)
	if old.GetInstanceName() != tc.GetInstanceName() {
		allErrs = append(allErrs, field.Invalid(field.NewPath("labels"), tc.Labels,
			"The instance must not be mutate or set value other than the cluster name"))
	}
	allErrs = append(allErrs, validateUpdatePDConfig(old.Spec.PD.Config, tc.Spec.PD.Config, field.NewPath("spec.pd.config"))...)
	allErrs = append(allErrs, disallowUsingLegacyAPIInNewCluster(old, tc)...)

	return allErrs
}

// For now we limit some validations only in Create phase to keep backward compatibility
// TODO(aylei): call this in ValidateTidbCluster after we deprecated the old versions of helm chart officially
func validateNewTidbClusterSpec(spec *v1alpha1.TidbClusterSpec, path *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	pdSpecified := spec.PD != nil
	tidbSpecified := spec.TiDB != nil
	tikvSpecified := spec.TiKV != nil

	if spec.Version == "" {
		allErrs = append(allErrs, field.Invalid(path.Child("version"), spec.Version, "version must not be empty"))
	}
	if tidbSpecified && spec.TiDB.BaseImage == "" {
		allErrs = append(allErrs, field.Invalid(path.Child("tidb.baseImage"), spec.TiDB.BaseImage, "baseImage of TiDB must not be empty"))
	}
	if pdSpecified && spec.PD.BaseImage == "" {
		allErrs = append(allErrs, field.Invalid(path.Child("pd.baseImage"), spec.PD.BaseImage, "baseImage of PD must not be empty"))
	}
	if tikvSpecified && spec.TiKV.BaseImage == "" {
		allErrs = append(allErrs, field.Invalid(path.Child("tikv.baseImage"), spec.TiKV.BaseImage, "baseImage of TiKV must not be empty"))
	}
	if tidbSpecified && spec.TiDB.Image != "" {
		allErrs = append(allErrs, field.Invalid(path.Child("tidb.image"), spec.TiDB.Image, "image has been deprecated, use baseImage instead"))
	}
	if tikvSpecified && spec.TiKV.Image != "" {
		allErrs = append(allErrs, field.Invalid(path.Child("tikv.image"), spec.TiKV.Image, "image has been deprecated, use baseImage instead"))
	}
	if pdSpecified && spec.PD.Image != "" {
		allErrs = append(allErrs, field.Invalid(path.Child("pd.image"), spec.PD.Image, "image has been deprecated, use baseImage instead"))
	}
	return allErrs
}

// disallowUsingLegacyAPIInNewCluster checks if user use the legacy API in newly create cluster during update
// TODO(aylei): this could be removed after we enable validateTidbCluster() in update, which is more strict
func disallowUsingLegacyAPIInNewCluster(old, tc *v1alpha1.TidbCluster) field.ErrorList {
	allErrs := field.ErrorList{}
	path := field.NewPath("spec")
	pdSpecified := old.Spec.PD != nil && tc.Spec.PD != nil
	tidbSpecified := old.Spec.TiDB != nil && tc.Spec.TiDB != nil
	tikvSpecified := old.Spec.TiKV != nil && tc.Spec.TiKV != nil

	if old.Spec.Version != "" && tc.Spec.Version == "" {
		allErrs = append(allErrs, field.Invalid(path.Child("version"), tc.Spec.Version, "version must not be empty"))
	}
	if tidbSpecified && old.Spec.TiDB.BaseImage != "" && tc.Spec.TiDB.BaseImage == "" {
		allErrs = append(allErrs, field.Invalid(path.Child("tidb.baseImage"), tc.Spec.TiDB.BaseImage, "baseImage of TiDB must not be empty"))
	}
	if pdSpecified && old.Spec.PD.BaseImage != "" && tc.Spec.PD.BaseImage == "" {
		allErrs = append(allErrs, field.Invalid(path.Child("pd.baseImage"), tc.Spec.PD.BaseImage, "baseImage of PD must not be empty"))
	}
	if tikvSpecified && old.Spec.TiKV.BaseImage != "" && tc.Spec.TiKV.BaseImage == "" {
		allErrs = append(allErrs, field.Invalid(path.Child("tikv.baseImage"), tc.Spec.TiKV.BaseImage, "baseImage of TiKV must not be empty"))
	}
	if tidbSpecified && old.Spec.TiDB.Config != nil && tc.Spec.TiDB.Config == nil {
		allErrs = append(allErrs, field.Invalid(path.Child("tidb.config"), tc.Spec.TiDB.Config, "tidb.config must not be nil"))
	}
	if tikvSpecified && old.Spec.TiKV.Config != nil && tc.Spec.TiKV.Config == nil {
		allErrs = append(allErrs, field.Invalid(path.Child("tikv.config"), tc.Spec.TiKV.Config, "TiKV.config must not be nil"))
	}
	if pdSpecified && old.Spec.PD.Config != nil && tc.Spec.PD.Config == nil {
		allErrs = append(allErrs, field.Invalid(path.Child("pd.config"), tc.Spec.PD.Config, "PD.config must not be nil"))
	}
	return allErrs
}

func validateUpdatePDConfig(old, conf *v1alpha1.PDConfigWraper, path *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	// for newly created cluster, both old and new are non-nil, guaranteed by validation
	if old == nil || conf == nil {
		return allErrs
	}

	if v := conf.Get("security.cert-allowed-cn"); v != nil {
		cn, err := v.AsStringSlice()
		if err != nil {
			allErrs = append(allErrs, field.Invalid(path.Child("security.cert-allowed-cn"), v.Interface(), err.Error()))
		} else if len(cn) > 1 {
			allErrs = append(allErrs, field.Invalid(path.Child("security.cert-allowed-cn"), v.Interface(),
				"Only one CN is currently supported"))
		}
	}

	oldSche := old.Get("schedule")
	newSche := conf.Get("schedule")
	if !reflect.DeepEqual(oldSche.Interface(), newSche.Interface()) {
		allErrs = append(allErrs, field.Invalid(path.Child("schedule"), newSche.Interface(),
			"PD Schedule Config is immutable through CRD, please modify with pd-ctl instead."))
	}

	oldRepl := old.Get("replication")
	newRepl := conf.Get("replication")
	if !reflect.DeepEqual(oldRepl, newRepl) {
		allErrs = append(allErrs, field.Invalid(path.Child("replication"), newRepl.Interface(),
			"PD Replication Config is immutable through CRD, please modify with pd-ctl instead."))
	}
	return allErrs
}

func validateDeleteSlots(annotations map[string]string, key string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if annotations != nil {
		if value, ok := annotations[key]; ok {
			var slice []int32
			err := json.Unmarshal([]byte(value), &slice)
			if err != nil {
				msg := fmt.Sprintf("value of %q annotation must be a JSON list of int32", key)
				allErrs = append(allErrs, field.Invalid(fldPath, value, msg))
			}
		}
	}
	return allErrs
}

func validateService(spec *v1alpha1.ServiceSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	//validate LoadBalancerSourceRanges field from service
	if len(spec.LoadBalancerSourceRanges) > 0 {
		ip := spec.LoadBalancerSourceRanges
		_, err := utilnet.ParseIPNets(ip...)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("spec.LoadBalancerSourceRanges"), spec.LoadBalancerSourceRanges, "service.Spec.LoadBalancerSourceRanges is not valid. Expecting a list of IP ranges. For example, 10.0.0.0/24."))
		}
	}
	return allErrs
}

// This validate will make sure targetPath:
// 1. is not abs path
// 2. does not have any element which is ".."
func validateLocalDescendingPath(targetPath string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if path.IsAbs(targetPath) {
		allErrs = append(allErrs, field.Invalid(fldPath, targetPath, "must be a relative path"))
	}

	allErrs = append(allErrs, validatePathNoBacksteps(targetPath, fldPath)...)

	return allErrs
}

// validatePathNoBacksteps makes sure the targetPath does not have any `..` path elements when split
//
// This assumes the OS of the apiserver and the nodes are the same. The same check should be done
// on the node to ensure there are no backsteps.
func validatePathNoBacksteps(targetPath string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	parts := strings.Split(filepath.ToSlash(targetPath), "/")
	for _, item := range parts {
		if item == ".." {
			allErrs = append(allErrs, field.Invalid(fldPath, targetPath, "must not contain '..'"))
			break // even for `../../..`, one error is sufficient to make the point
		}
	}
	return allErrs
}

func validateTimeDurationStr(timeStr *string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if timeStr != nil {
		d, err := time.ParseDuration(*timeStr)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath, timeStr, "mush be a valid Go time duration string, e.g. 3m"))
		} else if d <= 0 {
			allErrs = append(allErrs, field.Invalid(fldPath, timeStr, "must be a positive Go time duration"))
		}
	}
	return allErrs
}

// validatePromDurationStr validate prometheus duration, Units Supported: y, w, d, h, m, s, ms.
func validatePromDurationStr(timeStr *string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if timeStr != nil {
		if _, err := model.ParseDuration(*timeStr); err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath, timeStr, "mush be a valid Prom time duration string, e.g. 2h"))
		}
	}
	return allErrs
}

// clusterVersionLessThan2 makes sure that deployed dm cluster version not to be v1.0.x
func clusterVersionLessThan2(version string) (bool, error) {
	v, err := semver.NewVersion(version)
	if err != nil {
		return false, err
	}

	return v.Major() < 2, nil
}

func validateAdditionalContainers(containers []corev1.Container, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for i, container := range containers {
		idxPath := fldPath.Index(i)
		if len(container.Image) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("image"), "empty image"))
		}
	}

	return allErrs
}

func validateStorageInfo(storage string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(storage) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("storage"), storage, "storage must not be empty"))
	} else {
		_, err := resource.ParseQuantity(storage)
		if err != nil {
			allErrs = append(allErrs, &field.Error{
				Type:   field.ErrorTypeNotSupported,
				Detail: `value of "storage" format not supported`,
			})
		}
	}

	return allErrs
}
