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
	backoff "github.com/cenkalti/backoff/v4"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/GoogleCloudPlatform/sapagent/internal/cloudmonitoring"
	"github.com/GoogleCloudPlatform/sapagent/internal/commandlineexecutor"
	"github.com/GoogleCloudPlatform/sapagent/internal/processmetrics/sapcontrol"
	cnfpb "github.com/GoogleCloudPlatform/sapagent/protos/configuration"
	sapb "github.com/GoogleCloudPlatform/sapagent/protos/sapapp"
	"github.com/GoogleCloudPlatform/sapagent/shared/log"
)

const (
	nwCPUPath       = "/sap/nw/cpu/utilization"
	nwMemoryPath    = "/sap/nw/memory/utilization"
	nwIOPSReadsPath = "/sap/nw/iops/reads"
	nwIOPSWritePath = "/sap/nw/iops/writes"
)

type (
	// NetweaverInstanceProperties have the required context for collecting metrics for cpu and
	// memory per process for Netweaver.
	NetweaverInstanceProperties struct {
		Config                  *cnfpb.Configuration
		Client                  cloudmonitoring.TimeSeriesCreator
		Executor                commandlineexecutor.Execute
		SAPInstance             *sapb.SAPInstance
		NewProcHelper           newProcessWithContextHelper
		SAPControlProcessParams commandlineexecutor.Params
		ABAPProcessParams       commandlineexecutor.Params
		SAPControlClient        sapcontrol.ClientInterface
		LastValue               map[string]*process.IOCountersStat
		SkippedMetrics          map[string]bool
		PMBackoffPolicy         backoff.BackOffContext
	}
)

// Collect SAP additional metrics like per process CPU and per process memory
// utilization of SAP Netweaver processes.
// Collect method keeps on collecting all the metrics it can, logs errors if it encounters
// any and returns the collected metrics with the last error encountered while collecting metrics.
func (p *NetweaverInstanceProperties) Collect(ctx context.Context) ([]*mrpb.TimeSeries, error) {
	params := parameters{
		executor:             p.Executor,
		client:               p.Client,
		config:               p.Config,
		memoryMetricPath:     nwMemoryPath,
		cpuMetricPath:        nwCPUPath,
		iopsReadsMetricPath:  nwIOPSReadsPath,
		iopsWritesMetricPath: nwIOPSWritePath,
		lastValue:            p.LastValue,
		sapInstance:          p.SAPInstance,
		newProc:              p.NewProcHelper,
		getProcessListParams: p.SAPControlProcessParams,
		getABAPWPTableParams: p.ABAPProcessParams,
		SAPControlClient:     p.SAPControlClient,
	}
	processes := collectProcessesForInstance(ctx, params)
	var metricsCollectionErr error
	if len(processes) == 0 {
		log.Logger.Debug("cannot collect CPU and memory per process for Netweaver, empty process list.")
		return nil, nil
	}
	res := []*mrpb.TimeSeries{}
	if _, ok := p.SkippedMetrics[nwCPUPath]; !ok {
		cpuMetrics, err := collectCPUPerProcess(ctx, params, processes)
		if err != nil {
			metricsCollectionErr = err
		}
		if cpuMetrics != nil {
			res = append(res, cpuMetrics...)
		}
	}
	if _, ok := p.SkippedMetrics[nwMemoryPath]; !ok {
		memoryMetrics, err := collectMemoryPerProcess(ctx, params, processes)
		if err != nil {
			metricsCollectionErr = err
		}
		if memoryMetrics != nil {
			res = append(res, memoryMetrics...)
		}
	}
	skipIOPS := p.SkippedMetrics[nwIOPSReadsPath] || p.SkippedMetrics[nwIOPSWritePath]
	if !skipIOPS {
		iopsMetrics, err := collectIOPSPerProcess(ctx, params, processes)
		if err != nil {
			metricsCollectionErr = err
		}
		if iopsMetrics != nil {
			res = append(res, iopsMetrics...)
		}
	}
	return res, metricsCollectionErr
}

// CollectWithRetry decorates the Collect method with retry mechanism.
func (p *NetweaverInstanceProperties) CollectWithRetry(ctx context.Context) ([]*mrpb.TimeSeries, error) {
	var (
		attempt = 1
		res     []*mrpb.TimeSeries
	)
	err := backoff.Retry(func() error {
		var err error
		res, err = p.Collect(ctx)
		if err != nil {
			log.Logger.Errorw("Error in Collection", "attempt", attempt, "error", err)
			attempt++
		}
		return err
	}, p.PMBackoffPolicy)
	if err != nil {
		log.Logger.Debugw("Retry limit exceeded", "InstanceId", p.SAPInstance.GetInstanceId())
	}
	return res, err
}
