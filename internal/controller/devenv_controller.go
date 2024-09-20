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
	"crypto/rand"
	"encoding/hex"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	tanuudevv1alpha1 "github.com/tanuudev/tanuu-operator/api/v1alpha1"
)

const finalizerName = "tanuu.dev/finalizer"

// DevenvReconciler reconciles a Devenv object
type DevenvReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// generateRandomHash generates a random 4-character hash
func generateRandomHash() (string, error) {
	bytes := make([]byte, 2) // 2 bytes will give us 4 hex characters
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// +kubebuilder:rbac:groups=tanuu.dev,resources=devenvs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=tanuu.dev,resources=devenvs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=tanuu.dev,resources=devenvs/finalizers,verbs=update
// +kubebuilder:rbac:groups=tanuu.dev,resources=tanuunodes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// the Devenv object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.2/pkg/reconcile
func (r *DevenvReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)
	devenv := &tanuudevv1alpha1.Devenv{}
	err := r.Get(ctx, req.NamespacedName, devenv)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}
	if devenv.ObjectMeta.DeletionTimestamp == nil || devenv.ObjectMeta.DeletionTimestamp.IsZero() {
		// Resource is not being deleted
		if !containsString(devenv.ObjectMeta.Finalizers, finalizerName) {
			latestDevenv, err := r.getDevenvByNameAndNamespace(ctx, devenv.Name, devenv.Namespace)
			latestDevenv.ObjectMeta.Finalizers = append(devenv.ObjectMeta.Finalizers, finalizerName)

			if err != nil {
				l.Error(err, "unable to get Devenv by name and namespace")
				return ctrl.Result{}, err
			}
			// if err = r.Status().Update(ctx, latestDevenv); err != nil {
			if err = r.Update(ctx, latestDevenv); err != nil {
				// if err := r.Update(ctx, devenv); err != nil {
				l.Error(err, "unable to update Devenv with finalizer")
				return ctrl.Result{}, err
			}
		}
	} else {
		// Resource is being deleted

		if containsString(devenv.ObjectMeta.Finalizers, finalizerName) {
			// Run finalization logic for finalizerName.
			if err := r.delete_omni_cluster(ctx, r.Client, l, req, devenv); err != nil {
				return ctrl.Result{}, err
			}
			for _, node := range devenv.Status.ControlPlane {
				if err := r.deleteDevClusterNodes(ctx, devenv, node.Name); err != nil {
					return ctrl.Result{}, err
				}
			}
			for _, node := range devenv.Status.Workers {
				if err := r.deleteDevClusterNodes(ctx, devenv, node.Name); err != nil {
					return ctrl.Result{}, err
				}
			}
			for _, node := range devenv.Status.Gpus {
				if err := r.deleteDevClusterNodes(ctx, devenv, node.Name); err != nil {
					return ctrl.Result{}, err
				}
			}

			// Remove finalizer. Once all finalizers have been
			// removed, the object will be deleted.
			devenv.ObjectMeta.Finalizers = removeString(devenv.ObjectMeta.Finalizers, finalizerName)
			if err := r.Update(ctx, devenv); err != nil {
				return ctrl.Result{}, err
			}
		}
	}
	if devenv.Status.Status != "Ready" {
		// Mark as pending but not ready if it's a new object
		update := CopyDevenvUpdater(*devenv)

		if devenv.Status.Status == "" {
			// r.fetch_nodes_available(ctx, r.Client, l, req, devenv)
			devenv, err = r.getDevenvByNameAndNamespace(ctx, devenv.Name, devenv.Namespace)
			if err != nil {
				l.Error(err, "Failed to fetch Devenv object")
				return ctrl.Result{}, err
			}
			update.ControlPlane = devenv.Status.ControlPlane
			update.Workers = devenv.Status.Workers
			update.Gpus = devenv.Status.Gpus
			nodelist := []string{}
			for i := len(devenv.Status.ControlPlane); i < devenv.Spec.CtrlReplicas; i++ {
				NodeInfo, err := r.select_nodes(ctx, l, devenv.Spec.Name, "control", devenv.Spec.CtrlSelector, nodelist, devenv.Spec.StorageSelector)
				if err != nil {
					l.Error(err, "Failed to select node")
					return ctrl.Result{}, err
				}
				nodelist = append(nodelist, NodeInfo.UID)
				update.ControlPlane = append(update.ControlPlane, NodeInfo)
			}
			for i := len(devenv.Status.Workers); i < devenv.Spec.WorkerReplicas; i++ {
				NodeInfo, err := r.select_nodes(ctx, l, devenv.Spec.Name, "worker", devenv.Spec.WorkerSelector, nodelist, devenv.Spec.StorageSelector)
				if err != nil {
					l.Error(err, "Failed to select node")
					return ctrl.Result{}, err
				}
				nodelist = append(nodelist, NodeInfo.UID)
				update.Workers = append(update.Workers, NodeInfo)
			}

			if devenv.Spec.GpuReplicas > 0 {
				for i := len(devenv.Status.Gpus); i < devenv.Spec.GpuReplicas; i++ {
					NodeInfo, err := r.select_nodes(ctx, l, devenv.Spec.Name, "worker", devenv.Spec.GpuSelector, nodelist, devenv.Spec.StorageSelector)
					if err != nil {
						l.Error(err, "Failed to select node")
						return ctrl.Result{}, err
					}
					nodelist = append(nodelist, NodeInfo.UID)
					update.Gpus = append(update.Gpus, NodeInfo)
				}
				for _, node := range devenv.Status.Gpus {
					if node.UID == "" {
						createDevClusterNodes(ctx, r.Client, l, req, devenv, node.Name, "gpu")
					}
				}

			}
			update.Status = "GPUPending"

			r.Recorder.Event(devenv, "Normal", "Starting", "Devenv GPU provisioned.")
			if err := r.updateDevenvStatusWithRetry(ctx, devenv, update); err != nil {
				l.Error(err, "Failed to update Devenv status")
				return ctrl.Result{}, err
			}
		}
		if devenv.Status.Status == "GPUPending" {
			if devenv.Spec.GpuReplicas > 0 {
				// TODO create a check for GPU availability
				return ctrl.Result{RequeueAfter: time.Second * 5}, nil
			}

			for _, node := range devenv.Status.ControlPlane {
				if node.UID == "" {
					createDevClusterNodes(ctx, r.Client, l, req, devenv, node.Name, "control")
				}
			}
			for _, node := range devenv.Status.Workers {
				if node.UID == "" {
					createDevClusterNodes(ctx, r.Client, l, req, devenv, node.Name, "worker")
				}
			}
			update.Status = "Pending"
			r.Recorder.Event(devenv, "Normal", "Starting", "Devenv provisioned.")
			if err := r.updateDevenvStatusWithRetry(ctx, devenv, update); err != nil {
				l.Error(err, "Failed to update Devenv status")
				return ctrl.Result{}, err
			}
		}

		// Check if the Devenv is ready to be marked as Ready
		if devenv.Status.Status == "Scaling" {
			r.fetch_omni_nodes(ctx, r.Client, l, req, devenv)
			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}

		isReady, err := r.checkDevenvReadiness(ctx, r.Client, l, req, devenv)
		if err != nil {
			l.Error(err, "Failed to check Devenv readiness")
			return ctrl.Result{}, err
		}
		// r.update_omni_cluster(ctx, r.Client, l, req, devenv)
		if isReady {
			update.Status = "Ready"
			r.Recorder.Event(devenv, "Normal", "Ready", "Devenv ready.")
			if err := r.updateDevenvStatusWithRetry(ctx, devenv, update); err != nil {
				l.Error(err, "Failed to update Devenv status to Ready")
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		} else if devenv.Status.Status == "Pending" {
			r.fetch_omni_nodes(ctx, r.Client, l, req, devenv)
			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		} else if devenv.Status.Status == "Starting" {
			r.create_omni_cluster(ctx, r.Client, l, req, devenv)
			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}
	} else {
		// Devenv is ready, check if it needs to be updated
		update := CopyDevenvUpdater(*devenv)
		update.WorkerReplicas = len(devenv.Status.Workers)
		update.GpuReplicas = len(devenv.Status.Gpus)
		r.updateDevenvStatusWithRetry(ctx, devenv, update)
		if devenv.Spec.WorkerReplicas != update.WorkerReplicas {
			l.Info("Updating worker replicas")
			nodelist := []string{}
			if devenv.Spec.WorkerReplicas > update.WorkerReplicas {
				NodeInfo, err := r.select_nodes(ctx, l, devenv.Spec.Name, "worker", devenv.Spec.WorkerSelector, nodelist, devenv.Spec.StorageSelector)
				if err != nil {
					l.Error(err, "Failed to select node")
					return ctrl.Result{}, err
				}
				nodelist = append(nodelist, NodeInfo.UID)
				update.Workers = append(update.Workers, NodeInfo)
				if err := r.updateDevenvStatusWithRetry(ctx, devenv, update); err != nil {
					l.Error(err, "Failed to update Devenv status")
					return ctrl.Result{}, err
				}
				for _, node := range devenv.Status.Workers {
					if node.UID == "" {
						createDevClusterNodes(ctx, r.Client, l, req, devenv, node.Name, "worker")
					}
				}
				update.Status = "Scaling"
				r.updateDevenvStatusWithRetry(ctx, devenv, update)
				return ctrl.Result{RequeueAfter: time.Second * 5}, nil
			} else {
				// Scale down workers
				update.Status = "Scaling"
				// TODO delete the oldest nodes when scaling down
				// TODO delete non-pool resources first, i.e. the pool resources are dedicated full time, so they should be the last to go
			}
		}
		if devenv.Spec.GpuReplicas != update.GpuReplicas {
			// TODO copy from workers once working
			l.Info("Updating gpu replicas")
		}
		r.updateDevenvStatusWithRetry(ctx, devenv, update)
		if devenv.Status.Kubeconfig == "" {
			hostname := devenv.Spec.Name + "-ts"
			update.KubeConfig, err = r.createKubeConfig(ctx, r.Client, l, hostname)
			if err != nil {
				l.Error(err, "Failed to create kubeconfig")
				return ctrl.Result{}, err
			}
			r.updateDevenvStatusWithRetry(ctx, devenv, update)

		}
		return ctrl.Result{RequeueAfter: time.Second * 120}, nil
	}

	if err := r.Get(ctx, req.NamespacedName, devenv); err != nil {
		r.Recorder.Event(devenv, "Warning", "FetchFailed", "Failed to fetch Devenv object, likely deleted.")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	l.Info("Reconciled Devenv")
	r.Recorder.Event(devenv, "Normal", "Reconciled", "Devenv reconciled successfully")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DevenvReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tanuudevv1alpha1.Devenv{}).
		Complete(r)
}
