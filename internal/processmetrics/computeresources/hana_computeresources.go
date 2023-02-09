/*
Copyright 2022 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package computeresources

import (
	"context"

	"github.com/GoogleCloudPlatform/sapagent/internal/cloudmonitoring"
	"github.com/GoogleCloudPlatform/sapagent/internal/log"
	"github.com/GoogleCloudPlatform/sapagent/internal/processmetrics/sapcontrol"
	"github.com/GoogleCloudPlatform/sapagent/internal/processmetrics/sapdiscovery"
	cnfpb "github.com/GoogleCloudPlatform/sapagent/protos/configuration"
	sapb "github.com/GoogleCloudPlatform/sapagent/protos/sapapp"
)

const (
	hanaCPUPath    = "/sap/hana/cpu/utilization"
	hanaMemoryPath = "/sap/hana/memory/utilization"
)

type (
	// HanaInstanceProperties have the required context for collecting metrics for cpu
	// memory per process for HANA, Netweaver and SAP Control.
	// It also implements the InstanceProperiesIntrface for abstraction as defined in the
	// computreresources.go file.
	HanaInstanceProperties struct {
		Config        *cnfpb.Configuration
		Client        cloudmonitoring.TimeSeriesCreator
		Executor      commandExecutor
		SAPInstance   *sapb.SAPInstance
		Runner        sapcontrol.RunnerWithEnv
		NewProcHelper newProcessWithContextHelper
	}
)

// Collect SAP additional metrics like per process CPU and per process memory
// utilization of SAP HANA Processes.
func (p *HanaInstanceProperties) Collect(ctx context.Context) []*sapdiscovery.Metrics {
	params := parameters{
		executor:         p.Executor,
		client:           p.Client,
		config:           p.Config,
		memoryMetricPath: hanaMemoryPath,
		cpuMetricPath:    hanaCPUPath,
		sapInstance:      p.SAPInstance,
		newProc:          p.NewProcHelper,
		runnerForGetProcessList: p.Runner,
	}
	processes := collectProcessesForInstance(params)
	if len(processes) == 0 {
		log.Logger.Debug("Cannot collect CPU and memory per process for hana, empty process list.")
		return nil
	}
	return append(collectCPUPerProcess(ctx, params, processes), collectMemoryPerProcess(ctx, params, processes)...)
}
