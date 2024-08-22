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
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"strings"
	"text/template"

	"github.com/go-logr/logr"
	_ "github.com/lib/pq"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	k8client "sigs.k8s.io/controller-runtime/pkg/client"
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

func (r *DevenvReconciler) checkDevenvReadiness(ctx context.Context, ctrlclient k8client.Client, l logr.Logger, req ctrl.Request, devenv *tanuudevv1alpha1.Devenv) (bool, error) {
	return r.check_omni_cluster(ctx, r.Client, l, req, devenv)
}

// DevenvStatusUpdate represents the status information you want to update for a Devenv object.
type DevenvStatusUpdate struct {
	ControlPlane   []tanuudevv1alpha1.NodeInfo
	Workers        []tanuudevv1alpha1.NodeInfo
	Gpus           []tanuudevv1alpha1.NodeInfo
	IpAddress      string
	CloudProvider  string
	Status         string
	KubeConfig     string
	Services       []string
	WorkerReplicas int
	GpuReplicas    int
	WorkerSelector string
	CtrlSelector   string
	GpuSelector    string
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
		if update.ControlPlane != nil {
			latestDevenv.Status.ControlPlane = update.ControlPlane
		}
		if update.Workers != nil {
			latestDevenv.Status.Workers = update.Workers
		}
		if update.Gpus != nil {
			latestDevenv.Status.Gpus = update.Gpus
		}
		if update.IpAddress != "" {
			latestDevenv.Status.IpAddress = update.IpAddress
		}
		if update.CloudProvider != "" {
			latestDevenv.Status.CloudProvider = update.CloudProvider
		}
		if update.Status != "" {
			latestDevenv.Status.Status = update.Status
		}
		if update.KubeConfig != "" {
			latestDevenv.Status.Kubeconfig = update.KubeConfig
		}
		if update.Services != nil {
			latestDevenv.Status.Services = update.Services
		}
		if update.WorkerReplicas != 0 {
			latestDevenv.Status.WorkerReplicas = update.WorkerReplicas
		}
		if update.GpuReplicas != 0 {
			latestDevenv.Status.GpuReplicas = update.GpuReplicas
		} else {
			latestDevenv.Status.GpuReplicas = 0
		}

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

func (r *DevenvReconciler) getTailscaleHosts(ctx context.Context, devenv *tanuudevv1alpha1.Devenv) []string {
	// Read this from a secret
	connStr := "user=steampipe password=0255_4271_853e dbname=steampipe host=100.98.238.43 port=9193 sslmode=disable"
	// Connect to the database
	db, err := sql.Open("postgres", connStr)
	l := log.FromContext(ctx)
	if err != nil {
		l.Error(err, "failed to connect to the database")
	}
	defer db.Close()

	// SQL query
	sqlQuery := `
SELECT hostname, name, device.user
FROM tailscale_device as device
WHERE device.user is NULL;
`
	// Query the database
	rows, err := db.Query(sqlQuery)
	if err != nil {
		l.Error(err, "failed to query the database")
	}
	hosts := []string{}
	defer rows.Close()

	for row := rows; row.Next(); {
		var hostname string
		var host string
		var user sql.NullString
		err := row.Scan(&hostname, &host, &user)
		if err != nil {
			l.Error(err, "failed to scan the row")
		}

		if strings.Contains(host, devenv.Spec.Name) {
			l.Info("Tailscale device", "hostname", hostname, "name", host)
			hosts = append(hosts, host)
		}
	}
	return hosts
}

type KubeConfig struct {
	Host    string
	Tailnet string
}

func (r *DevenvReconciler) createKubeConfig(ctx context.Context, ctrlclient k8client.Client, l logr.Logger, host string) (string, error) {
	configMap := &corev1.ConfigMap{}
	// Define the namespaced name to look up the ConfigMap
	namespacedName := types.NamespacedName{
		Namespace: "tanuu-system",
		Name:      "cluster",
	}
	// Get the ConfigMap
	if err := ctrlclient.Get(ctx, namespacedName, configMap); err != nil {
		l.Error(err, "Failed to get ConfigMap")
		return "", err
	}

	// Extract the configuration string
	tailnet, _ := configMap.Data["tailnet.name"]
	const kubeConfigTemplate = `
apiVersion: v1
clusters:
- cluster:
    server: https://{{ .Host }}.{{ .Tailnet }}
  name: {{ .Host }}
contexts:
- context:
    cluster: {{ .Host }}
    namespace: default
    user: tailscale-auth
  name: {{ .Host }}
current-context: {{ .Host }}
kind: Config
preferences: {}
users:
- name: tailscale-auth
  user:
    token: unused
`
	config := KubeConfig{
		Host:    host,
		Tailnet: tailnet,
	}
	tmpl, err := template.New("kubeConfig").Parse(kubeConfigTemplate)
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	err = tmpl.Execute(&out, config)
	if err != nil {
		return "", err
	}

	return out.String(), nil
}
