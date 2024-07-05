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

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/version"
	"google.golang.org/protobuf/types/known/emptypb"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func test_omni(ctx context.Context) {
	l := log.FromContext(ctx)
	// This example shows how to use Omni client to access resources.

	// Setup versions information. You can embed that into `go build` too.
	version.Name = "omni"
	version.SHA = "build SHA"
	version.Tag = "v0.9.1"

	// For this example we will use Omni service account.
	// You can create your service account in advance:
	//
	// omnictl serviceaccount create example.account
	// Created service account "example.account" with public key ID "<REDACTED>"
	//
	// Set the following environment variables to use the service account:
	// OMNI_ENDPOINT=https://<account>.omni.siderolabs.io:443
	// OMNI_SERVICE_ACCOUNT_KEY=base64encodedkey
	//
	// Note: Store the service account key securely, it will not be displayed again
	// Creating a new client.
	client, err := client.New("insert_url_here_from_secret", client.WithServiceAccount("insert_token_here_from_secret")) // From the generated service account.

	if err != nil {
		l.Error(err, "failed to create omni client %s", err)
	}

	// Omni service is using COSI https://github.com/cosi-project/runtime/.
	// The same client is used to get resources in Talos.
	st := client.Omni().State()

	// Getting the resources from the Omni state.
	machines, err := safe.StateList[*omni.MachineStatus](ctx, st, omni.NewMachineStatus(resources.DefaultNamespace, "").Metadata())
	if err != nil {
		l.Error(err, "failed to get machines %s", err)
	}

	var (
		cluster string
		machine *omni.MachineStatus
	)

	for iter := safe.IteratorFromList(machines); iter.Next(); {
		item := iter.Value()

		l.Info("machine %s, connected: %t", item.TypedSpec().Value.Network.Hostname, item.TypedSpec().Value.GetConnected())

		// Check cluster assignment for a machine.
		// Find a machine which is allocated into a cluster for the later use.
		if c, ok := item.Metadata().Labels().Get(omni.LabelCluster); ok && machine == nil {
			cluster = c
			machine = item
		}
	}

	// No machines found, exit.
	if machine == nil {

		return
	}
	cpuInfo, err := client.Talos().WithCluster(
		cluster,
	).WithNodes(
		machine.Metadata().ID(), // You can use machine UUID as Omni will properly resolve it into machine IP.
	).CPUInfo(ctx, &emptypb.Empty{})
	if err != nil {
		l.Error(err, "failed to read machine CPU info %s", err)
	}

	for _, message := range cpuInfo.Messages {
		for i, info := range message.CpuInfo {
			l.Info("machine %s, CPU %d family %s", machine.Metadata(), i, info.CpuFamily)
		}

		if len(message.CpuInfo) == 0 {
			l.Info("no CPU info for machine %s", machine.Metadata())
		}
	}

	// Talking to Omni specific APIs: getting talosconfig.
	_, err = client.Management().Talosconfig(ctx)
	if err != nil {
		l.Error(err, "failed to get talosconfig %s", err)
	}
}
