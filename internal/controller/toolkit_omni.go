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
	"strings"
	templ "text/template"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/go-logr/logr"
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/template"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	k8client "sigs.k8s.io/controller-runtime/pkg/client"

	tanuudevv1alpha1 "github.com/tanuudev/tanuu-operator/api/v1alpha1"
)

type Environment struct {
	Name                  string
	ControlPlane          string
	Workers               string
	Gpus                  string
	TailScaleClientID     string
	TailScaleClientSecret string
	GitHubToken           string
	Gpu                   bool
}

var clustertempl *templ.Template

func (r *DevenvReconciler) fetch_omni_nodes(ctx context.Context, ctrlclient k8client.Client, l logr.Logger, req ctrl.Request, devenv *tanuudevv1alpha1.Devenv) error {
	secret := &corev1.Secret{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: "omni-creds", Namespace: "tanuu-system"}, secret)
	if err != nil {
		l.Error(err, "unable to fetch creds from secret")
		return err
	}
	// version.Name = "omni"
	// version.SHA = "build SHA"
	// version.Tag = "v0.9.1"

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
		typedSpec := item.TypedSpec()
		if typedSpec == nil {
			return nil // or continue, depending on the context
		}
		typedSpecValue := typedSpec.Value
		if typedSpecValue == nil {
			return nil // or continue
		}
		network := typedSpecValue.Network
		if network == nil {
			return nil // or continue
		}
		if strings.Contains(item.TypedSpec().Value.Network.Hostname, devenv.Spec.Name) {
			l.Info("machine " + item.TypedSpec().Value.Network.Hostname + ", found: ")
			if strings.Contains(item.TypedSpec().Value.Network.Hostname, "control") {
				update.ControlPlane = append(update.ControlPlane, "  - "+item.Metadata().ID())
			}
			if strings.Contains(item.TypedSpec().Value.Network.Hostname, "worker") {
				update.Workers = append(update.Workers, "  - "+item.Metadata().ID())
			}
			if strings.Contains(item.TypedSpec().Value.Network.Hostname, "gpu") {
				update.Gpus = append(update.Gpus, "  - "+item.Metadata().ID())
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
	err := r.Client.Get(ctx, types.NamespacedName{Name: "omni-creds", Namespace: "tanuu-system"}, secret)
	if err != nil {
		l.Error(err, "unable to fetch creds from secret")
		return err
	}
	// Define the ConfigMap object
	configMap := &corev1.ConfigMap{}
	// Define the namespaced name to look up the ConfigMap
	namespacedName := types.NamespacedName{
		Namespace: "tanuu-system",
		Name:      "cluster",
	}
	// Get the ConfigMap
	if err := ctrlclient.Get(ctx, namespacedName, configMap); err != nil {
		l.Error(err, "Failed to get ConfigMap")
		return err
	}

	// Extract the configuration string
	configString, exists := configMap.Data["cluster.tmpl"]
	if !exists {
		panic("Specified key does not exist in the ConfigMap")
	}

	environment := Environment{}
	var buf bytes.Buffer
	environment.Name = devenv.Spec.Name
	environment.ControlPlane = strings.Join(devenv.Status.ControlPlane, "\n")
	environment.Workers = strings.Join(devenv.Status.Workers, "\n")
	environment.Gpus = strings.Join(devenv.Status.Gpus, "\n")
	environment.TailScaleClientID = string(secret.Data["TailScaleClientID"])
	environment.TailScaleClientSecret = string(secret.Data["TailScaleClientSecret"])
	environment.GitHubToken = string(secret.Data["GitHubToken"])
	clustertempl = templ.Must(templ.New("cluster").Parse(configString))
	// l.Info(clustertempl.Root.String())
	err = clustertempl.Execute(&buf, environment)
	if err != nil {
		l.Error(err, "Failed to execute template")
		return err
	}
	// version.Name = "omni"
	// version.SHA = "build SHA"
	// version.Tag = "v0.9.1"

	url := string(secret.Data["url"])
	token := string(secret.Data["token"])

	client, err := client.New(url, client.WithServiceAccount(token)) // From the generated service account.
	if err != nil {
		l.Error(err, "failed to create omni client")
		return err
	}
	st := client.Omni().State()
	templ1, err := template.Load(bytes.NewReader(buf.Bytes()))
	if err != nil {
		l.Error(err, "failed to create omni client")
		return err
	}
	sync1, err := templ1.Sync(ctx, st)
	if err != nil {
		l.Error(err, "failed to create omni client")
		return err
	}
	for _, r := range sync1.Create {

		if err = st.Create(ctx, r); err != nil {
			return err
		}
	}
	l.Info("Creating cluster")
	return nil

}

func (r *DevenvReconciler) check_omni_cluster(ctx context.Context, ctrlclient k8client.Client, l logr.Logger, req ctrl.Request, devenv *tanuudevv1alpha1.Devenv) (bool, error) {
	secret := &corev1.Secret{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: "omni-creds", Namespace: "tanuu-system"}, secret)
	if err != nil {
		l.Error(err, "unable to fetch creds from secret")
		return false, err
	}

	url := string(secret.Data["url"])
	token := string(secret.Data["token"])

	client, err := client.New(url, client.WithServiceAccount(token)) // From the generated service account.
	if err != nil {
		l.Error(err, "failed to create omni client")
		return false, err
	}
	st := client.Omni().State()

	clusters, err := safe.StateList[*omni.ClusterStatus](ctx, st, omni.NewClusterStatus(resources.DefaultNamespace, "").Metadata())
	if err != nil {
		l.Error(err, "failed to get machines %s", err)
		return false, err
	}
	for iter := safe.IteratorFromList(clusters); iter.Next(); {
		item := iter.Value()
		if item.Metadata().ID() == devenv.Spec.Name {
			typedSpec := item.TypedSpec()
			if typedSpec == nil {
				return false, nil // or continue, depending on the context
			}
			typedSpecValue := typedSpec.Value
			if typedSpecValue == nil {
				return false, nil // or continue
			}
			if typedSpecValue.Ready {
				return true, nil
			}
		}

	}
	return false, nil

}

// TODO Delete cluster nodes also
func (r *DevenvReconciler) delete_omni_cluster(ctx context.Context, ctrlclient k8client.Client, l logr.Logger, req ctrl.Request, devenv *tanuudevv1alpha1.Devenv) error {
	secret := &corev1.Secret{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: "omni-creds", Namespace: "tanuu-system"}, secret)
	if err != nil {
		l.Error(err, "unable to fetch creds from secret")
		return err
	}
	// Define the ConfigMap object
	configMap := &corev1.ConfigMap{}
	// Define the namespaced name to look up the ConfigMap
	namespacedName := types.NamespacedName{
		Namespace: "tanuu-system",
		Name:      "cluster",
	}
	// Get the ConfigMap
	if err := ctrlclient.Get(ctx, namespacedName, configMap); err != nil {
		l.Error(err, "Failed to get ConfigMap")
		return err
	}

	// Extract the configuration string
	configString, exists := configMap.Data["cluster.tmpl"]
	if !exists {
		panic("Specified key does not exist in the ConfigMap")
	}

	environment := Environment{}
	var buf bytes.Buffer
	environment.Name = devenv.Spec.Name
	environment.ControlPlane = strings.Join(devenv.Status.ControlPlane, "\n")
	environment.Workers = strings.Join(devenv.Status.Workers, "\n")
	environment.Gpus = strings.Join(devenv.Status.Gpus, "\n")
	// LEAVE these as null, as they are not used for deletion
	environment.TailScaleClientID = "null"
	environment.TailScaleClientSecret = "null"
	environment.GitHubToken = "null"
	clustertempl = templ.Must(templ.New("cluster").Parse(configString))
	// l.Info(clustertempl.Root.String())
	err = clustertempl.Execute(&buf, environment)
	if err != nil {
		l.Error(err, "Failed to execute template")
		return err
	}

	url := string(secret.Data["url"])
	token := string(secret.Data["token"])

	client, err := client.New(url, client.WithServiceAccount(token)) // From the generated service account.
	if err != nil {
		l.Error(err, "failed to create omni client")
		return err
	}
	st := client.Omni().State()
	templ1, err := template.Load(bytes.NewReader(buf.Bytes()))
	if err != nil {
		l.Error(err, "failed to create omni client")
		return err
	}
	syncDelete, err := templ1.Delete(ctx, st)
	if err != nil {
		l.Error(err, "failed to create omni client")
		return err
	}
	for _, r := range syncDelete.Destroy {
		for _, item := range r {
			if item != nil {
				if _, err = st.Teardown(ctx, item.Metadata()); err != nil {
					return err
				}
			}
		}
	}
	l.Info("Deleting cluster")
	return nil

}
