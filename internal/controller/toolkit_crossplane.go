/*
Copyright 2024 tanuudev.

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

package controller

import (
	"context"
	"strconv"
	"strings"
	"unicode"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	tanuudevv1alpha1 "github.com/tanuudev/tanuu-operator/api/v1alpha1"
)

// stripNonDigits removes all non-digit characters from the input string.
func stripNonDigits(input string) string {
	var builder strings.Builder
	for _, char := range input {
		if unicode.IsDigit(char) {
			builder.WriteRune(char)
		}
	}
	return builder.String()
}

func parseConfigString(config string) map[string]interface{} {
	lines := strings.Split(config, "\n")
	result := make(map[string]interface{})
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			valueStr := strings.TrimSpace(parts[1])
			// Attempt to parse the value as an integer
			if intValue, err := strconv.Atoi(valueStr); err == nil {
				// If successful, use the integer value
				result[key] = intValue
			} else {
				// Otherwise, use the string value
				result[key] = valueStr
			}
		}
	}
	return result
}

func createDevClusterNodes(ctx context.Context, client client.Client, l logr.Logger, req ctrl.Request, devenv *tanuudevv1alpha1.Devenv, nodename string, nodetype string) {
	nodetypeselector := ""
	machineType := ""
	// Define the ConfigMap object
	tanuuConfigMap := &corev1.ConfigMap{}
	// Define the namespaced name to look up the ConfigMap
	configmapname := types.NamespacedName{
		Namespace: "tanuu-system",
		Name:      "tanuu-operator",
	}
	// Get the ConfigMap
	if err := client.Get(ctx, configmapname, tanuuConfigMap); err != nil {
		l.Error(err, "Failed to get ConfigMap")
		return
	}
	// Extract the configuration string
	defalutWorkerSelector, exists := tanuuConfigMap.Data["default.workerSelector"]
	defaultCtrlSelector, exists := tanuuConfigMap.Data["default.ctrlSelector"]
	defaultGpuSelector, exists := tanuuConfigMap.Data["default.gpuSelector"]
	defaultTalosVersion, exists := tanuuConfigMap.Data["default.talosVersion"]
	defaultCtrlMachineType, exists := tanuuConfigMap.Data["default.ctrlMachineType"]
	defaultWorkerMachineType, exists := tanuuConfigMap.Data["default.workerMachineType"]
	defaultGpuMachineType, exists := tanuuConfigMap.Data["default.gpuMachineType"]
	defaultServiceAccount, exists := tanuuConfigMap.Data["default.serviceAccount"]
	defaultSubnetwork, exists := tanuuConfigMap.Data["default.subnetwork"]

	if !exists {
		panic("Specified key does not exist in the ConfigMap")
	}
	if devenv.Spec.Subnetwork == "" {
		devenv.Spec.Subnetwork = defaultSubnetwork
	}
	if devenv.Spec.ServiceAccount == "" {
		devenv.Spec.ServiceAccount = defaultServiceAccount
	}
	if devenv.Spec.TalosVersion == "" {
		devenv.Spec.TalosVersion = defaultTalosVersion
	}
	if devenv.Spec.ProviderConfig == "" {
		devenv.Spec.ProviderConfig = "default"
	}

	if nodetype == "worker" {
		if devenv.Spec.WorkerMachineType == "" {
			machineType = defaultWorkerMachineType
		}
		if devenv.Spec.WorkerSelector == "" {
			nodetypeselector = defalutWorkerSelector
		} else {
			nodetypeselector = devenv.Spec.WorkerSelector
		}
	} else if nodetype == "control" {
		if devenv.Spec.CtrlMachineType == "" {
			machineType = defaultCtrlMachineType
		}
		if devenv.Spec.CtrlSelector == "" {
			nodetypeselector = defaultCtrlSelector
		} else {
			nodetypeselector = devenv.Spec.CtrlSelector
		}
	} else if nodetype == "gpu" {
		if devenv.Spec.GpuMachineType == "" {
			machineType = defaultGpuMachineType
		}
		if devenv.Spec.GpuSelector == "" {
			nodetypeselector = defaultGpuSelector
		} else {
			nodetypeselector = devenv.Spec.GpuSelector
		}
	}
	customResource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "tanuu.dev/v1beta1",
			"kind":       "TanuuNode",
			"metadata": map[string]interface{}{
				"name":      nodename,
				"namespace": "tanuu-system",
			},
			"spec": map[string]interface{}{
				"compositionSelector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"provider": devenv.Spec.CloudProvider,
						"nodetype": nodetypeselector},
				},
				"parameters": map[string]interface{}{
					"zone":           devenv.Spec.Zone,
					"providerConfig": devenv.Spec.ProviderConfig,
					"talosVersion":   stripNonDigits(devenv.Spec.TalosVersion),
					"subnetwork":     devenv.Spec.Subnetwork,
					"serviceAccount": devenv.Spec.ServiceAccount,
					"machineType":    machineType},
			},
		},
	}
	if err := client.Create(ctx, customResource); err != nil {
		if errors.IsAlreadyExists(err) {
			// If the error is because the resource already exists, ignore it.
		} else {
			// Log other types of errors.
			l.Error(err, "unable to create custom resource")
		}
	} else {
		l.Info("custom resource created successfully")
	}
}

func (r *DevenvReconciler) deleteDevClusterNodes(ctx context.Context, devenv *tanuudevv1alpha1.Devenv, nodename string) error {
	l := log.FromContext(ctx)
	c := r.Client
	// deleteDevCluster(ctx context.Context, client client.Client, l logr.Logger, req ctrl.Request)
	// Define the custom resource to delete
	customResource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "tanuu.dev/v1beta1",
			"kind":       "TanuuNode",
			"metadata": map[string]interface{}{
				"name":      nodename,
				"namespace": "tanuu-system",
			},
		},
	}
	// Attempt to delete the custom resource
	if err := c.Delete(ctx, customResource); err != nil {
		if errors.IsNotFound(err) {
			// If the error is because the resource does not exist, ignore it.
			l.Info("custom resource already deleted or not found")
		} else {
			// Log other types of errors.
			l.Error(err, "unable to delete custom resource")
		}
	} else {
		l.Info("custom resource deleted successfully")
	}
	return nil
}
