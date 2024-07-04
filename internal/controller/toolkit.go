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
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	tanuudevv1alpha1 "github.com/tanuudev/tanuu-operator/api/v1alpha1"
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
		// update := DevenvStatusUpdate{}
		// update.IpAddress = "127.0.0.1:8080"
		// update.CloudProvider = devenv.Spec.CloudProvider
		// update.ControlPlane = []string{"123e4567-e89b-12d3-a456-426614174000", "123e4567-e89b-12d3-a456-426614174001"}
		// update.Workers = []string{"123e4567-e89b-12d3-a456-426614174002", "123e4567-e89b-12d3-a456-426614174003"}
		// update.Gpus = []string{"123e4567-e89b-12d3-a456-426614174004", "123e4567-e89b-12d3-a456-426614174005"}
		// r.updateDevenvStatusWithRetry(ctx, devenv, update)
		// if err := r.Status().Update(ctx, devenv); err != nil {
		// 	l.Error(err, "unable to update Devenv status")
		// 	return ctrl.Result{}, err
		// }
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

func (r *DevenvReconciler) checkDevenvReadiness(ctx context.Context, devenv *tanuudevv1alpha1.Devenv) (bool, error) {
	// Example readiness check logic
	// This should be replaced with actual checks relevant to your Devenv object
	return false, nil
}

// DevenvStatusUpdate represents the status information you want to update for a Devenv object.
type DevenvStatusUpdate struct {
	ControlPlane  []string
	Workers       []string
	Gpus          []string
	IpAddress     string
	CloudProvider string
	Status        string
}

// updateDevenvStatusWithRetry updates the status of a Devenv object with retry logic.
func (r *DevenvReconciler) updateDevenvStatusWithRetry(ctx context.Context, devenv *tanuudevv1alpha1.Devenv, update DevenvStatusUpdate) error {
	logger := log.FromContext(ctx)
	retryAttempts := 3
	for i := 0; i < retryAttempts; i++ {
		latestDevenv, err := r.getDevenvByNameAndNamespace(ctx, devenv.Name, devenv.Namespace)
		if err != nil {
			logger.Error(err, "Failed to fetch the latest version of Devenv for update", "Devenv", devenv.Name)
			return err
		}

		// Apply the updates to the latest version of the Devenv object
		latestDevenv.Status.ControlPlane = update.ControlPlane
		latestDevenv.Status.Workers = update.Workers
		latestDevenv.Status.Gpus = update.Gpus
		latestDevenv.Status.IpAddress = update.IpAddress
		latestDevenv.Status.CloudProvider = update.CloudProvider
		latestDevenv.Status.Status = update.Status

		err = r.Status().Update(ctx, latestDevenv)
		if err != nil {
			if errors.IsConflict(err) {
				logger.Info("Conflict error occurred during Devenv status update. Retrying...", "attempt", i+1)
				continue // Retry the operation
			}
			logger.Error(err, "Failed to update Devenv status")
			return err // Handle non-retryable errors appropriately
		}
		return nil // Success
	}
	return fmt.Errorf("update failed after %d attempts due to conflict errors", retryAttempts)
}

func (r *DevenvReconciler) getDevenvByNameAndNamespace(ctx context.Context, name, namespace string) (*tanuudevv1alpha1.Devenv, error) {
	devenv := &tanuudevv1alpha1.Devenv{}
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, devenv)
	if err != nil {
		return nil, err
	}
	return devenv, nil
}
