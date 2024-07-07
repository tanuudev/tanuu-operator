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
	"context"
	"strings"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/go-logr/logr"
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/version"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	k8client "sigs.k8s.io/controller-runtime/pkg/client"

	tanuudevv1alpha1 "github.com/tanuudev/tanuu-operator/api/v1alpha1"
)

func (r *DevenvReconciler) test_omni(ctx context.Context, ctrlclient k8client.Client, l logr.Logger, req ctrl.Request, devenv *tanuudevv1alpha1.Devenv) {
	secret := &corev1.Secret{}
	// TODO make the secret namespace and name variables
	err := r.Client.Get(ctx, types.NamespacedName{Name: "omni-creds", Namespace: "default"}, secret)
	if err != nil {
		l.Error(err, "unable to fetch creds from secret")
		return
	}
	version.Name = "omni"
	version.SHA = "build SHA"
	version.Tag = "v0.9.1"

	url := string(secret.Data["url"])
	token := string(secret.Data["token"])

	client, err := client.New(url, client.WithServiceAccount(token)) // From the generated service account.

	if err != nil {
		l.Error(err, "failed to create omni client %s", err)
	}

	st := client.Omni().State()

	machines, err := safe.StateList[*omni.MachineStatus](ctx, st, omni.NewMachineStatus(resources.DefaultNamespace, "").Metadata())
	if err != nil {
		l.Error(err, "failed to get machines %s", err)
	}

	for iter := safe.IteratorFromList(machines); iter.Next(); {
		item := iter.Value()
		if strings.Contains(item.ResourceDefinition().DefaultNamespace, devenv.Spec.Name) {
			l.Info("machine " + item.TypedSpec().Value.Network.Hostname + ", found: ")
		}

	}

}
