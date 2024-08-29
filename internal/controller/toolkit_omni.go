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

// NOTE: Taken from https://pkg.go.dev/github.com/siderolabs/omni-client/pkg/client#example-package

package controller

import (
	"bytes"
	"context"
	"strings"
	templ "text/template"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/go-logr/logr"
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/template"
	"github.com/siderolabs/omni/client/pkg/template/operations"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	k8client "sigs.k8s.io/controller-runtime/pkg/client"

	tanuudevv1alpha1 "github.com/tanuudev/tanuu-operator/api/v1alpha1"
)

type Environment struct {
	Name                    string
	ControlPlane            string
	Workers                 string
	Gpus                    string
	TailScaleClientID       string
	TailScaleClientSecret   string
	GitHubToken             string
	Gpu                     bool
	K8sVersion              string
	TalosVersion            string
	OnePasswordConnectToken string
}

var clustertempl *templ.Template

func (r *DevenvReconciler) fetch_omni_nodes(ctx context.Context, ctrlclient k8client.Client, l logr.Logger, req ctrl.Request, devenv *tanuudevv1alpha1.Devenv) error {
	secret := &corev1.Secret{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: "omni-creds", Namespace: "tanuu-system"}, secret)
	if err != nil {
		l.Error(err, "unable to fetch creds from secret")
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

	machines, err := safe.StateList[*omni.MachineStatus](ctx, st, omni.NewMachineStatus(resources.DefaultNamespace, "").Metadata())
	if err != nil {
		l.Error(err, "failed to get machines")
		return err
	}
	update := CopyDevenvUpdater(*devenv)
	for iter := machines.Iterator(); iter.Next(); {
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
		for _, node := range devenv.Status.ControlPlane {
			if strings.Contains(item.TypedSpec().Value.Network.Hostname, node.Name) {
				node.UID = item.Metadata().ID()
				node.CreatedAt = time.Now().String()
				for i, updateNode := range update.ControlPlane {
					if updateNode.Name == node.Name {
						update.ControlPlane[i] = node
						break
					}
				}
			}
		}
		for _, node := range devenv.Status.Workers {
			if strings.Contains(item.TypedSpec().Value.Network.Hostname, node.Name) {
				node.UID = item.Metadata().ID()
				node.CreatedAt = time.Now().String()
				for i, updateNode := range update.Workers {
					if updateNode.Name == node.Name {
						update.Workers[i] = node
						break
					}
				}
			}
		}
		for _, node := range devenv.Status.Gpus {
			if strings.Contains(item.TypedSpec().Value.Network.Hostname, node.Name) {
				node.UID = item.Metadata().ID()
				node.CreatedAt = time.Now().String()
				for i, updateNode := range update.Gpus {
					if updateNode.Name == node.Name {
						update.Gpus[i] = node
						break
					}
				}
			}
		}
	}
	nodesReady := 0
	nodeCount := devenv.Spec.CtrlReplicas + devenv.Spec.WorkerReplicas + devenv.Spec.GpuReplicas
	for _, node := range update.ControlPlane {
		if node.UID != "" {
			nodesReady++
		}
	}
	for _, node := range update.Workers {
		if node.UID != "" {
			nodesReady++
		}
	}
	for _, node := range update.Gpus {
		if node.UID != "" {
			nodesReady++
		}
	}

	if nodesReady == nodeCount {
		update.Status = "Starting"
		if err := r.updateDevenvStatusWithRetry(ctx, devenv, update); err != nil {
			l.Error(err, "Failed to update Devenv status to Ready")
			return err
		}
		r.Recorder.Event(devenv, "Normal", "Starting", "Nodes started.")
	}

	return nil

}

func (r *DevenvReconciler) select_nodes(ctx context.Context, l logr.Logger, env_name string, group string, selector string) (tanuudevv1alpha1.NodeInfo, error) {
	secret := &corev1.Secret{}
	nodeinfo := tanuudevv1alpha1.NodeInfo{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: "omni-creds", Namespace: "tanuu-system"}, secret)
	if err != nil {
		l.Error(err, "unable to fetch creds from secret")
		return nodeinfo, err
	}

	url := string(secret.Data["url"])
	token := string(secret.Data["token"])

	client, err := client.New(url, client.WithServiceAccount(token)) // From the generated service account.

	if err != nil {
		l.Error(err, "failed to create omni client")
		return nodeinfo, err
	}

	st := client.Omni().State()

	machines, err := safe.StateList[*omni.MachineStatus](ctx, st, omni.NewMachineStatus(resources.DefaultNamespace, "").Metadata())
	if err != nil {
		l.Error(err, "failed to get machines")
		return nodeinfo, err
	}
	for iter := machines.Iterator(); iter.Next(); {
		item := iter.Value()
		typedSpec := item.TypedSpec()
		typedMetadata := item.Metadata()
		typedMetadataValue := typedMetadata.Labels()
		if typedMetadata == nil {
			return nodeinfo, err // or continue, depending on the context
		}
		if typedMetadataValue == nil {
			return nodeinfo, err // or continue
		}
		if typedSpec == nil {
			return nodeinfo, err // or continue, depending on the context
		}
		typedSpecValue := typedSpec.Value
		if typedSpecValue == nil {
			return nodeinfo, err // or continue
		}
		// labels := typedSpecValue.ImageLabels
		labels := typedMetadataValue.KV
		cluster := typedSpecValue.Cluster
		if cluster != "" {
			for key, value := range labels.Raw() {
				if key == "pool" && value == selector {
					//TODO fix this to use the correct nodeinfo
					// The aim here is that we could have a pool of physical nodes that can be used as needed as a pool
					// for the devenv. This is useful for example when we have a pool of nodes with GPUs.
					// Selection here might work, needs to check if the node is already in use, is connected and available, and matches the selector.
					nodeinfo.Name = typedMetadata.ID()
					nodeinfo.CreatedAt = "pool-" + time.Now().String()
					nodeinfo.UID = typedMetadata.ID()
					l.Info("Found node with pool label " + value)
					return nodeinfo, nil
				}
			}
		}

	}
	hash, err := generateRandomHash()
	if err != nil {
		l.Error(err, "Failed to generate random hash")
		return nodeinfo, err
	}
	nodeinfo.Name = env_name + "-" + group + "-" + hash
	return nodeinfo, nil
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
	environment.K8sVersion = devenv.Spec.K8sVersion
	environment.TalosVersion = devenv.Spec.TalosVersion
	controlplanes := []string{}
	workers := []string{}
	gpus := []string{}
	for _, node := range devenv.Status.ControlPlane {
		controlplanes = append(controlplanes, "  - "+node.UID)
	}
	for _, node := range devenv.Status.Workers {
		workers = append(workers, "  - "+node.UID)
	}
	for _, node := range devenv.Status.Gpus {
		gpus = append(gpus, "  - "+node.UID)
	}

	environment.ControlPlane = strings.Join(controlplanes, "\n")
	environment.Workers = strings.Join(workers, "\n")
	environment.Gpus = strings.Join(gpus, "\n")
	environment.TailScaleClientID = string(secret.Data["TailScaleClientID"])
	environment.TailScaleClientSecret = string(secret.Data["TailScaleClientSecret"])
	environment.GitHubToken = string(secret.Data["GitHubToken"])
	environment.OnePasswordConnectToken = string(secret.Data["OnePasswordConnectToken"])
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
		l.Error(err, "failed to get machines")
		return false, err
	}
	for iter := clusters.Iterator(); iter.Next(); {
		item := iter.Value()
		if item.Metadata().ID() == devenv.Spec.Name {
			typedSpec := item.TypedSpec()
			// if num machines != all replicas count then false
			if typedSpec == nil {
				return false, nil // or continue, depending on the context
			}
			typedSpecValue := typedSpec.Value
			if typedSpecValue == nil {
				return false, nil // or continue
			}
			totalreplicas := devenv.Spec.CtrlReplicas + devenv.Spec.WorkerReplicas + devenv.Spec.GpuReplicas
			if typedSpecValue.Machines.Healthy != uint32(totalreplicas) {
				return false, nil
			}
			if typedSpecValue.Ready {
				return true, nil
			}
		}

	}
	return false, nil

}

func (r *DevenvReconciler) delete_omni_cluster(ctx context.Context, ctrlclient k8client.Client, l logr.Logger, req ctrl.Request, devenv *tanuudevv1alpha1.Devenv) error {

	secret := &corev1.Secret{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: "omni-creds", Namespace: "tanuu-system"}, secret)
	if err != nil {
		l.Error(err, "unable to fetch creds from secret")
		return err
	}

	url := string(secret.Data["url"])
	token := string(secret.Data["token"])

	client, err := client.New(url, client.WithServiceAccount(token)) // From the generated service account.
	if err != nil {
		l.Error(err, "failed to get omni client")
		return err
	}
	st := client.Omni().State()

	clusters, err := safe.StateList[*omni.ClusterStatus](ctx, st, omni.NewClusterStatus(resources.DefaultNamespace, "").Metadata())
	if err != nil {
		l.Error(err, "failed to get cluster.")
		return err
	}
	machines, err := safe.StateList[*omni.MachineStatus](ctx, st, omni.NewMachineStatus(resources.DefaultNamespace, "").Metadata())
	if err != nil {
		l.Error(err, "failed to delete omni client")
	}
	for iter := machines.Iterator(); iter.Next(); {
		item := iter.Value()
		// loop through the devenv controplane nodes and delete them
		for _, node := range devenv.Status.ControlPlane {
			if item.Metadata().ID() == node.UID {
				// if _, err = st.Teardown(ctx, item.Metadata()); err != nil {
				if _, err = st.Teardown(ctx, resource.NewMetadata(item.ResourceDefinition().DefaultNamespace, "Links.omni.sidero.dev", item.Metadata().ID(), resource.VersionUndefined)); err != nil {
					return err
				}
			}
		}
		for _, node := range devenv.Status.Workers {
			if item.Metadata().ID() == node.UID {
				// if _, err = st.Teardown(ctx, item.Metadata()); err != nil {
				if _, err = st.Teardown(ctx, resource.NewMetadata(item.ResourceDefinition().DefaultNamespace, "Links.omni.sidero.dev", item.Metadata().ID(), resource.VersionUndefined)); err != nil {
					return err
				}
			}
		}
		for _, node := range devenv.Status.Gpus {
			if item.Metadata().ID() == node.UID {
				// if _, err = st.Teardown(ctx, item.Metadata()); err != nil {
				if _, err = st.Teardown(ctx, resource.NewMetadata(item.ResourceDefinition().DefaultNamespace, "Links.omni.sidero.dev", item.Metadata().ID(), resource.VersionUndefined)); err != nil {
					return err
				}
			}
		}
	}
	templ1 := &template.Template{}
	for iter := clusters.Iterator(); iter.Next(); {
		item := iter.Value()

		if item.Metadata().ID() == devenv.Spec.Name {
			var output bytes.Buffer
			_, err = operations.ExportTemplate(ctx, st, devenv.Spec.Name, &output)
			if err != nil {
				l.Error(err, "failed to get cluster")
				return err
			}
			templ1, err = template.Load(bytes.NewReader(output.Bytes()))
			if err != nil {
				l.Error(err, "failed to get cluster")
				return err
			}
			clusterName, err := templ1.ClusterName()
			if err != nil {
				l.Error(err, "failed to get cluster")
				return err
			}
			l.Info("Found cluster:" + clusterName)

		}
	}
	syncDelete, err := templ1.Delete(ctx, st)

	if err != nil {
		l.Error(err, "failed to delete omni client")
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
	// Getting the resources from the Omni state.

	return nil

}
