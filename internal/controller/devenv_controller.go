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

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
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
			// For additional cleanup logic use finalizers.
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}
	if devenv.ObjectMeta.DeletionTimestamp.IsZero() {
		// Resource is not being deleted
		if !containsString(devenv.ObjectMeta.Finalizers, finalizerName) {
			devenv.ObjectMeta.Finalizers = append(devenv.ObjectMeta.Finalizers, finalizerName)
			if err := r.Update(ctx, devenv); err != nil {
				l.Error(err, "unable to update Devenv with finalizer")
				return ctrl.Result{}, err
			}
		}
	} else {
		// Resource is being deleted
		if containsString(devenv.ObjectMeta.Finalizers, finalizerName) {
			// Run finalization logic for finalizerName. If the
			// finalization logic fails, don't remove the finalizer so
			// that we can retry during the next reconciliation.
			if err := r.deleteDevCluster(ctx, devenv); err != nil {
				return ctrl.Result{}, err
			}

			// Remove finalizer. Once all finalizers have been
			// removed, the object will be deleted.
			devenv.ObjectMeta.Finalizers = removeString(devenv.ObjectMeta.Finalizers, finalizerName)
			if err := r.Update(ctx, devenv); err != nil {
				return ctrl.Result{}, err
			}
		}
	}
	if err := r.Get(ctx, req.NamespacedName, devenv); err != nil {
		r.Recorder.Event(devenv, "Warning", "FetchFailed", "Failed to fetch Devenv object, likely deleted.")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	r.Recorder.Event(devenv, "Normal", "Starting", "Devenv object started reconciliation.")

	createDevCluster(ctx, r.Client, l, req)

	devenv.Status.IpAddress = "127.0.0.1:8080"
	devenv.Status.CloudProvider = devenv.Spec.CloudProvider
	devenv.Status.ControlPLane = []string{"123e4567-e89b-12d3-a456-426614174000", "123e4567-e89b-12d3-a456-426614174001"}
	devenv.Status.Workers = []string{"123e4567-e89b-12d3-a456-426614174002", "123e4567-e89b-12d3-a456-426614174003"}
	devenv.Status.Gpus = []string{"123e4567-e89b-12d3-a456-426614174004", "123e4567-e89b-12d3-a456-426614174005"}
	if err := r.Status().Update(ctx, devenv); err != nil {
		l.Error(err, "unable to update Devenv status")
		return ctrl.Result{}, err
	}
	l.Info("Reconciled Devenv")
	r.Recorder.Event(devenv, "Normal", "Reconciled", "Devenv object reconciled successfully")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DevenvReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tanuudevv1alpha1.Devenv{}).
		Complete(r)
}

// func (r *DevenvReconciler) doCleanup(ctx context.Context, devenv *tanuudevv1alpha1.Devenv) error {
// 	// Implement your clean-up logic here
// 	// For example, deleting associated resources or releasing external resources
// 	l := log.FromContext(ctx)
// 	deleteDevCluster(ctx, r.Client, l, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(devenv)})
// 	// If clean-up is successful, return nil
// 	return nil
// }
