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

// NOTE: Taken from https://pkg.go.dev/github.com/siderolabs/omni-client/pkg/client#example-package

package controller

import (
	"bytes"
	"context"
	"strconv"
	"strings"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/go-logr/logr"
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/template"
	"github.com/siderolabs/omni/client/pkg/version"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	k8client "sigs.k8s.io/controller-runtime/pkg/client"

	tanuudevv1alpha1 "github.com/tanuudev/tanuu-operator/api/v1alpha1"
)

func (r *DevenvReconciler) fetch_omni_nodes(ctx context.Context, ctrlclient k8client.Client, l logr.Logger, req ctrl.Request, devenv *tanuudevv1alpha1.Devenv) error {
	secret := &corev1.Secret{}
	// TODO make the secret namespace and name variables
	err := r.Client.Get(ctx, types.NamespacedName{Name: "omni-creds", Namespace: "default"}, secret)
	if err != nil {
		l.Error(err, "unable to fetch creds from secret")
		return err
	}
	version.Name = "omni"
	version.SHA = "build SHA"
	version.Tag = "v0.9.1"

	url := string(secret.Data["url"])
	token := string(secret.Data["token"])

	client, err := client.New(url, client.WithServiceAccount(token)) // From the generated service account.

	if err != nil {
		l.Error(err, "failed to create omni client %s", err)
		return err
	}

	st := client.Omni().State()

	machines, err := safe.StateList[*omni.MachineStatus](ctx, st, omni.NewMachineStatus(resources.DefaultNamespace, "").Metadata())
	if err != nil {
		l.Error(err, "failed to get machines %s", err)
		return err
	}
	update := &DevenvStatusUpdate{}
	for iter := safe.IteratorFromList(machines); iter.Next(); {
		item := iter.Value()
		if strings.Contains(item.TypedSpec().Value.Network.Hostname, devenv.Spec.Name) {
			l.Info("machine " + item.TypedSpec().Value.Network.Hostname + ", found: ")
			if strings.Contains(item.TypedSpec().Value.Network.Hostname, "control") {
				update.ControlPlane = append(update.ControlPlane, item.Metadata().ID())
			}
			if strings.Contains(item.TypedSpec().Value.Network.Hostname, "worker") {
				update.Workers = append(update.Workers, item.Metadata().ID())
			}
			if strings.Contains(item.TypedSpec().Value.Network.Hostname, "gpu") {
				update.Gpus = append(update.Gpus, item.Metadata().ID())
			}
		}

	}
	allConditionsMet := true

	if len(update.ControlPlane) != 1 {
		l.Info("control plane not found")
		allConditionsMet = false
	}
	// TODO make the number of worker nodes a variable
	if len(update.Workers) != 2 {
		l.Info("worker nodes not found")
		allConditionsMet = false
	}
	// TODO make the gpu nodes optional and variable
	if devenv.Spec.Gpu && len(update.Gpus) != 1 {
		l.Info("gpu nodes not found")
		allConditionsMet = false
	}

	if allConditionsMet {
		update.Status = "Starting"
		if err := r.updateDevenvStatusWithRetry(ctx, devenv, *update); err != nil {
			l.Error(err, "Failed to update Devenv status to Ready")
			return err
		}
		r.Recorder.Event(devenv, "Normal", "Starting", "Nodes started.")
	}

	return nil

}

func (r *DevenvReconciler) create_omni_cluster(ctx context.Context, ctrlclient k8client.Client, l logr.Logger, req ctrl.Request, devenv *tanuudevv1alpha1.Devenv) error {
	secret := &corev1.Secret{}
	// TODO make the secret namespace and name variables
	err := r.Client.Get(ctx, types.NamespacedName{Name: "omni-creds", Namespace: "default"}, secret)
	if err != nil {
		l.Error(err, "unable to fetch creds from secret")
		return err
	}
	version.Name = "omni"
	version.SHA = "build SHA"
	version.Tag = "v0.9.1"

	url := string(secret.Data["url"])
	token := string(secret.Data["token"])

	client, err := client.New(url, client.WithServiceAccount(token)) // From the generated service account.
	data := []byte{}
	template.Load(bytes.NewReader(data))
	if err != nil {
		l.Error(err, "failed to create omni client %s", err)
		return err
	}

	st := client.Omni().State()
	machines, err := safe.StateList[*omni.MachineStatus](ctx, st, omni.NewMachineStatus(resources.DefaultNamespace, "").Metadata())
	if err != nil {
		l.Error(err, "failed to get machines %s", err)
		return err
	}

	l.Info("Creating cluster" + strconv.Itoa(machines.Len()))
	return nil

}
