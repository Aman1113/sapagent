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

	mrpb "google.golang.org/genproto/googleapis/monitoring/v3"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/GoogleCloudPlatform/sapagent/internal/cloudmonitoring"
	"github.com/GoogleCloudPlatform/sapagent/internal/commandlineexecutor"
	"github.com/GoogleCloudPlatform/sapagent/internal/processmetrics/sapcontrol"
	cnfpb "github.com/GoogleCloudPlatform/sapagent/protos/configuration"
	sapb "github.com/GoogleCloudPlatform/sapagent/protos/sapapp"
	"github.com/GoogleCloudPlatform/sapagent/shared/log"
)

const (
	hanaCPUPath        = "/sap/hana/cpu/utilization"
	hanaMemoryPath     = "/sap/hana/memory/utilization"
	hanaIOPSReadsPath  = "/sap/hana/iops/reads"
	hanaIOPSWritesPath = "/sap/hana/iops/writes"
)

type (
	// HanaInstanceProperties have the required context for collecting metrics for cpu
	// memory per process for HANA, Netweaver and SAP Control.
	// It also implements the InstanceProperiesInterface for abstraction as defined in the
	// computreresources.go file.
	HanaInstanceProperties struct {
		Config            *cnfpb.Configuration
		Client            cloudmonitoring.TimeSeriesCreator
		Executor          commandlineexecutor.Execute
		SAPInstance       *sapb.SAPInstance
		ProcessListParams commandlineexecutor.Params
		LastValue         map[string]*process.IOCountersStat
		NewProcHelper     newProcessWithContextHelper
		SAPControlClient  sapcontrol.ClientInterface
		SkippedMetrics    map[string]bool
	}
)

// Collect SAP additional metrics like per process CPU and per process memory
// utilization of SAP HANA Processes.
func (p *HanaInstanceProperties) Collect(ctx context.Context) []*mrpb.TimeSeries {
	params := parameters{
		executor:             p.Executor,
		client:               p.Client,
		config:               p.Config,
		memoryMetricPath:     hanaMemoryPath,
		cpuMetricPath:        hanaCPUPath,
		iopsReadsMetricPath:  hanaIOPSReadsPath,
		iopsWritesMetricPath: hanaIOPSWritesPath,
		lastValue:            p.LastValue,
		sapInstance:          p.SAPInstance,
		newProc:              p.NewProcHelper,
		getProcessListParams: p.ProcessListParams,
		SAPControlClient:     p.SAPControlClient,
	}
	processes := collectProcessesForInstance(ctx, params)
	if len(processes) == 0 {
		log.Logger.Debug("Cannot collect CPU and memory per process for hana, empty process list.")
		return nil
	}
	res := make([]*mrpb.TimeSeries, 0)
	if _, ok := p.SkippedMetrics[hanaCPUPath]; !ok {
		res = append(res, collectCPUPerProcess(ctx, params, processes)...)
	}
	if _, ok := p.SkippedMetrics[hanaMemoryPath]; !ok {
		res = append(res, collectMemoryPerProcess(ctx, params, processes)...)
	}
	skipIOPS := p.SkippedMetrics[hanaIOPSReadsPath] || p.SkippedMetrics[hanaIOPSWritesPath]
	if !skipIOPS {
		res = append(res, collectIOPSPerProcess(ctx, params, processes)...)
	}
	return res
}
