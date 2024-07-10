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

// +kubebuilder:rbac:groups=tanuu.dev.envs,resources=devenvs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=tanuu.dev.envs,resources=devenvs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=tanuu.dev.envs,resources=devenvs/finalizers,verbs=update
// +kubebuilder:rbac:groups=tanuu.dev,resources=nodegroupclaims,verbs=get;list;watch;create;update;patch;delete
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
			if err := r.deleteDevClusterNodes(ctx, devenv, "worker"); err != nil {
				return ctrl.Result{}, err
			}
			if err := r.deleteDevClusterNodes(ctx, devenv, "control"); err != nil {
				return ctrl.Result{}, err
			}
			if devenv.Spec.Gpu {
				if err := r.deleteDevClusterNodes(ctx, devenv, "gpu"); err != nil {
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
		update := DevenvStatusUpdate{}
		if devenv.Status.Status == "" {
			update.Status = "Pending"
			createDevClusterNodes(ctx, r.Client, l, req, devenv, "control")
			createDevClusterNodes(ctx, r.Client, l, req, devenv, "worker")
			if devenv.Spec.Gpu {
				createDevClusterNodes(ctx, r.Client, l, req, devenv, "gpu")
			}

			r.Recorder.Event(devenv, "Normal", "Starting", "Devenv provisioned.")
			if err := r.updateDevenvStatusWithRetry(ctx, devenv, update); err != nil {
				l.Error(err, "Failed to update Devenv status")
				return ctrl.Result{}, err
			}
		}

		// Check if the Devenv is ready to be marked as Ready

		isReady, err := r.checkDevenvReadiness(ctx, r.Client, l, req, devenv)
		if err != nil {
			l.Error(err, "Failed to check Devenv readiness")
			return ctrl.Result{}, err
		}

		if isReady {
			update.Status = "Ready"
			r.Recorder.Event(devenv, "Normal", "Ready", "Devenv ready.")
			if err := r.updateDevenvStatusWithRetry(ctx, devenv, update); err != nil {
				l.Error(err, "Failed to update Devenv status to Ready")
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: time.Second * 30}, nil
		} else if devenv.Status.Status == "Pending" {
			r.fetch_omni_nodes(ctx, r.Client, l, req, devenv)
			return ctrl.Result{RequeueAfter: time.Second * 30}, nil
		} else if devenv.Status.Status == "Starting" {
			r.create_omni_cluster(ctx, r.Client, l, req, devenv)
			return ctrl.Result{RequeueAfter: time.Second * 30}, nil
		}
	} else {
		// Devenv is ready, check if it needs to be updated
		// ts_hosts := r.getTailscaleHosts(ctx, devenv)
		r.getTailscaleHosts(ctx, devenv)
		return ctrl.Result{RequeueAfter: time.Second * 300}, nil
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
