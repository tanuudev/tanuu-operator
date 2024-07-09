/*
Copyright 2024 punasusi.

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

func createDevClusterNodes(ctx context.Context, client client.Client, l logr.Logger, req ctrl.Request, devenv *tanuudevv1alpha1.Devenv, nodetype string) {
	// Define the ConfigMap object
	configMap := &corev1.ConfigMap{}
	// Define the namespaced name to look up the ConfigMap
	namespacedName := types.NamespacedName{
		Namespace: "tanuu-system",
		Name:      "node-configs",
	}
	// Get the ConfigMap
	if err := client.Get(ctx, namespacedName, configMap); err != nil {
		l.Error(err, "Failed to get ConfigMap")
		return
	}

	// Extract the configuration string
	configString, exists := configMap.Data[nodetype]
	if !exists {
		panic("Specified key does not exist in the ConfigMap")
	}
	customResource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "tanuu.dev/v1alpha1",
			"kind":       "NodeGroupClaim",
			"metadata": map[string]interface{}{
				"name":      devenv.Spec.Name + "-" + nodetype + "-group",
				"namespace": req.Namespace,
			},
			"spec": map[string]interface{}{
				"compositionSelector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"provider": devenv.Spec.CloudProvider,
						"cluster":  "gke"},
				},
				"id":         devenv.Spec.Name + "-" + nodetype + "-group",
				"parameters": parseConfigString(configString),
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

func (r *DevenvReconciler) deleteDevClusterNodes(ctx context.Context, devenv *tanuudevv1alpha1.Devenv, nodetype string) error {
	l := log.FromContext(ctx)
	c := r.Client
	req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(devenv)}
	// deleteDevCluster(ctx context.Context, client client.Client, l logr.Logger, req ctrl.Request)
	// Define the custom resource to delete
	customResource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "tanuu.dev/v1alpha1",
			"kind":       "NodeGroupClaim",
			"metadata": map[string]interface{}{
				"name":      devenv.Spec.Name + "-" + nodetype + "-group",
				"namespace": req.Namespace,
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
