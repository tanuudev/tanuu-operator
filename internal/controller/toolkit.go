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

	"github.com/go-logr/logr"
	tanuudevv1alpha1 "github.com/tanuudev/tanuu-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) []string {
	result := []string{}
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return result
}

func createDevCluster(ctx context.Context, client client.Client, l logr.Logger, req ctrl.Request) {
	customResource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "tanuu.dev/v1alpha1",
			"kind":       "NodeGroupClaim",
			"metadata": map[string]interface{}{
				"name":      "andy-test-worker-group",
				"namespace": req.Namespace,
			},
			"spec": map[string]interface{}{
				"compositionSelector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"provider": "google",
						"cluster":  "gke",
					},
				},
				"id": "andy-test-worker-group",
				"parameters": map[string]interface{}{
					"replicas":            2,
					"size":                50,
					"image":               "projects/silogen-sandbox/global/images/omni-worker-v5",
					"imageType":           "projects/silogen-sandbox/zones/europe-west4-a/diskTypes/pd-balanced",
					"machineType":         "e2-highmem-4",
					"serviceAccountEmail": "1067721308413-compute@developer.gserviceaccount.com",
					"zone":                "europe-west4-a",
				},
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

func (r *DevenvReconciler) deleteDevCluster(ctx context.Context, devenv *tanuudevv1alpha1.Devenv) error {
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
				"name":      "andy-test-worker-group",
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