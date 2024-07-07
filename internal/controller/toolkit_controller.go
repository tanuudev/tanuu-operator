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

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
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

func (r *DevenvReconciler) checkDevenvReadiness(ctx context.Context, devenv *tanuudevv1alpha1.Devenv) (bool, error) {
	// Example readiness check logic
	// This should be replaced with actual checks relevant to your Devenv object
	// TODO check the nodes are running, and cluster is available
	return false, nil
}

func (r *DevenvReconciler) checkDevenvNodesReadiness(ctx context.Context, devenv *tanuudevv1alpha1.Devenv) (bool, error) {
	// Example readiness check logic
	// This should be replaced with actual checks relevant to your Devenv object
	// TODO check the nodes are running, and cluster is available
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
