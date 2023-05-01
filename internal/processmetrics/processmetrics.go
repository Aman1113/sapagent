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

// Package processmetrics provides process metrics collection capability for GC SAP Agent.
// This package is the landing space for process metrics collection in SAP GC Agent.
// This package abstracts the underlying metric collectors from the caller. The package
// is responsible for discovering the SAP applications running on this machine, and
// starting the relevant metric collectors in background.
// The package is responsible for the lifecycle of process metric specific jobs which involves:
//   - Create asynchronous jobs using the configuration passed by the caller.
//   - Monitor the job statuses and communicate with other components of GC SAP Agent.
//   - Restart the jobs that fail to ensure we continue to push the metrics.
//   - Send telemetry on aggregated job stats.
//
// The package defines and consumes Collector interface. Each individual metric collection file -
// hana, netweaver, cluster etc need to implement this interface to leverage the common
// collectMetrics and sendMetrics functions.
package processmetrics

import (
	"context"
	"fmt"
	"sync"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"github.com/GoogleCloudPlatform/sapagent/internal/cloudmonitoring"
	"github.com/GoogleCloudPlatform/sapagent/internal/commandlineexecutor"
	"github.com/GoogleCloudPlatform/sapagent/internal/heartbeat"
	"github.com/GoogleCloudPlatform/sapagent/internal/log"
	"github.com/GoogleCloudPlatform/sapagent/internal/processmetrics/cluster"
	"github.com/GoogleCloudPlatform/sapagent/internal/processmetrics/computeresources"
	"github.com/GoogleCloudPlatform/sapagent/internal/processmetrics/hana"
	"github.com/GoogleCloudPlatform/sapagent/internal/processmetrics/infra"
	"github.com/GoogleCloudPlatform/sapagent/internal/processmetrics/maintenance"
	"github.com/GoogleCloudPlatform/sapagent/internal/processmetrics/netweaver"
	"github.com/GoogleCloudPlatform/sapagent/internal/processmetrics/sapdiscovery"
	"github.com/GoogleCloudPlatform/sapagent/internal/processmetrics/sapservice"
	"github.com/GoogleCloudPlatform/sapagent/internal/usagemetrics"

	mrpb "google.golang.org/genproto/googleapis/monitoring/v3"
	cpb "github.com/GoogleCloudPlatform/sapagent/protos/configuration"
	sapb "github.com/GoogleCloudPlatform/sapagent/protos/sapapp"
)

type (
	// Collector interface is SAP application specific metric collection logic.
	// This needs to be implennted by application specific modules that want to leverage
	// startMetricGroup functionality.
	Collector interface {
		Collect(context.Context) []*mrpb.TimeSeries
	}

	// Properties has necessary context for Metrics collection.
	Properties struct {
		SAPInstances  *sapb.SAPInstances // Optional for production use cases, used by unit tests.
		Config        *cpb.Configuration
		Client        cloudmonitoring.TimeSeriesCreator
		Collectors    []Collector
		HeartbeatSpec *heartbeat.Spec
	}

	// CreateMetricClient provides an easily testable translation to the cloud monitoring API.
	CreateMetricClient func(ctx context.Context) (cloudmonitoring.TimeSeriesCreator, error)

	// Parameters has parameters necessary to invoke Start().
	Parameters struct {
		Config        *cpb.Configuration
		OSType        string
		MetricClient  CreateMetricClient
		SAPInstances  *sapb.SAPInstances
		BackOffs      *cloudmonitoring.BackOffIntervals
		HeartbeatSpec *heartbeat.Spec
		GCEService    sapdiscovery.GCEInterface
	}
)

const (
	maxTSPerRequest  = 200 // Reference: https://cloud.google.com/monitoring/quotas
	countReset       = 10000
	bufferSize       = 10000
	minimumFrequency = 5
)

/*
Start starts collection if collect_process_metrics config option is enabled
in the configuration. The function is a NO-OP if the config option is not enabled.

If the config option is enabled, start the metric collector jobs
in the background and return control to the caller with return value = true.

Return false if the config option is not enabled.
*/
func Start(ctx context.Context, parameters Parameters) bool {
	cpm := parameters.Config.GetCollectionConfiguration().GetCollectProcessMetrics()
	cf := parameters.Config.GetCollectionConfiguration().GetProcessMetricsFrequency()

	log.Logger.Infow("Configuration option for collect_process_metrics", "collectprocessmetrics", cpm)

	switch {
	case !cpm:
		log.Logger.Info("Not collecting Process Metrics.")
		return false
	case parameters.OSType == "windows":
		log.Logger.Info("Process Metrics collection is not supported for windows platform.")
		return false
	case cf < minimumFrequency:
		log.Logger.Infow("Process metrics frequency is smaller than minimum supported value.", "frequency", cf, "minimumfrequency", minimumFrequency)
		log.Logger.Info("Not collecting Process Metrics.")
		return false
	}

	mc, err := parameters.MetricClient(ctx)
	if err != nil {
		log.Logger.Errorw("Failed to create Cloud Monitoring client", "error", err)
		usagemetrics.Error(usagemetrics.ProcessMetricsMetricClientCreateFailure) // Failed to create Cloud Monitoring client
		return false
	}

	sapInstances := instancesWithCredentials(ctx, &parameters)
	if len(sapInstances.GetInstances()) == 0 {
		log.Logger.Error("No SAP Instances found. Cannot start process metrics collection.")
		usagemetrics.Error(usagemetrics.NoSAPInstancesFound) // NO SAP instances found
		return false
	}

	log.Logger.Info("Starting process metrics collection in background.")
	go usagemetrics.LogActionDaily(usagemetrics.CollectProcessMetrics)
	p := create(ctx, parameters, mc, sapInstances)
	// NOMUTANTS--will be covered by integration testing
	go p.collectAndSend(ctx, parameters.BackOffs)
	return true
}

// NewMetricClient is the production version that calls cloud monitoring API.
func NewMetricClient(ctx context.Context) (cloudmonitoring.TimeSeriesCreator, error) {
	return monitoring.NewMetricClient(ctx)
}

// create sets up the processmetrics properties and metric collectors for SAP Instances.
func create(ctx context.Context, params Parameters, client cloudmonitoring.TimeSeriesCreator, sapInstances *sapb.SAPInstances) *Properties {
	p := &Properties{
		SAPInstances:  sapInstances,
		Config:        params.Config,
		Client:        client,
		HeartbeatSpec: params.HeartbeatSpec,
	}

	log.Logger.Info("Creating SAP additional metrics collector for sapservices (active and enabled metric).")
	sapServiceCollector := &sapservice.InstanceProperties{
		Config:  p.Config,
		Client:  p.Client,
		Execute: commandlineexecutor.ExecuteCommand,
	}

	log.Logger.Info("Creating SAP control processes per process CPU, memory usage metrics collector.")
	sapStartCollector := &computeresources.SAPControlProcInstanceProperties{
		Config:   p.Config,
		Client:   p.Client,
		Executor: commandlineexecutor.ExecuteCommand,
	}

	log.Logger.Info("Creating infra migration event metrics collector.")
	migrationCollector := &infra.Properties{
		Config: p.Config,
		Client: p.Client,
	}

	p.Collectors = append(p.Collectors, sapServiceCollector, sapStartCollector, migrationCollector)

	sids := make(map[string]bool)
	clusterCollectorCreated := false
	for _, instance := range p.SAPInstances.GetInstances() {
		sids[instance.GetSapsid()] = true
		if p.SAPInstances.GetLinuxClusterMember() && clusterCollectorCreated == false {
			log.Logger.Infow("Creating cluster collector for instance", "instance", instance)
			clusterCollector := &cluster.InstanceProperties{
				SAPInstance: instance,
				Config:      p.Config,
				Client:      p.Client,
			}
			p.Collectors = append(p.Collectors, clusterCollector)
			clusterCollectorCreated = true
		}
		if instance.GetType() == sapb.InstanceType_HANA {
			log.Logger.Infow("Creating HANA per process CPU, memory usage metrics collector for instance", "instance", instance)
			hanaComputeresourcesCollector := &computeresources.HanaInstanceProperties{
				Config:      p.Config,
				Client:      p.Client,
				Executor:    commandlineexecutor.ExecuteCommand,
				SAPInstance: instance,
				ProcessListParams: commandlineexecutor.Params{
					User:        instance.GetUser(),
					Executable:  instance.GetSapcontrolPath(),
					ArgsToSplit: fmt.Sprintf("-nr %s -function GetProcessList -format script", instance.GetInstanceNumber()),
					Env:         []string{"LD_LIBRARY_PATH=" + instance.GetLdLibraryPath()},
				},
			}

			log.Logger.Infow("Creating HANA collector for instance.", "instance", instance)
			hanaCollector := &hana.InstanceProperties{
				SAPInstance:        instance,
				Config:             p.Config,
				Client:             p.Client,
				HANAQueryFailCount: 0,
			}
			p.Collectors = append(p.Collectors, hanaComputeresourcesCollector, hanaCollector)
		}
		if instance.GetType() == sapb.InstanceType_NETWEAVER {
			log.Logger.Infow("Creating Netweaver per process CPU, memory usage metrics collector for instance.", "instance", instance)
			netweaverComputeresourcesCollector := &computeresources.NetweaverInstanceProperties{
				Config:      p.Config,
				Client:      p.Client,
				Executor:    commandlineexecutor.ExecuteCommand,
				SAPInstance: instance,
				SAPControlProcessParams: commandlineexecutor.Params{
					User:        instance.GetUser(),
					Executable:  instance.GetSapcontrolPath(),
					ArgsToSplit: fmt.Sprintf("-nr %s -function GetProcessList -format script", instance.GetInstanceNumber()),
					Env:         []string{"LD_LIBRARY_PATH=" + instance.GetLdLibraryPath()},
				},
				ABAPProcessParams: commandlineexecutor.Params{
					User:        instance.GetUser(),
					Executable:  instance.GetSapcontrolPath(),
					ArgsToSplit: fmt.Sprintf("-nr %s -function ABAPGetWPTable", instance.GetInstanceNumber()),
					Env:         []string{"LD_LIBRARY_PATH=" + instance.GetLdLibraryPath()},
				},
			}

			log.Logger.Infow("Creating Netweaver collector for instance.", "instance", instance)
			netweaverCollector := &netweaver.InstanceProperties{
				SAPInstance: instance,
				Config:      p.Config,
				Client:      p.Client,
			}
			p.Collectors = append(p.Collectors, netweaverComputeresourcesCollector, netweaverCollector)
		}
	}

	if len(sids) != 0 {
		log.Logger.Info("Creating maintenance mode collector.")
		maintenanceModeCollector := &maintenance.InstanceProperties{
			Config: p.Config,
			Client: p.Client,
			Reader: maintenance.ModeReader{},
			Sids:   sids,
		}
		p.Collectors = append(p.Collectors, maintenanceModeCollector)
	}

	log.Logger.Infow("Created process metrics collectors.", "numberofcollectors", len(p.Collectors))
	return p
}

// instancesWithCredentials run SAP discovery to detect SAP instances on this machine and update
// DB Credentials from configuration into the instances.
func instancesWithCredentials(ctx context.Context, params *Parameters) *sapb.SAPInstances {
	// For unit tests we do not want to run sap discovery, caller will pass the SAPInstances.
	if params.SAPInstances == nil {
		params.SAPInstances = sapdiscovery.SAPApplications()
	}
	for _, instance := range params.SAPInstances.GetInstances() {
		if instance.GetType() == sapb.InstanceType_HANA {
			var err error
			projectID := params.Config.GetCloudProperties().GetProjectId()
			hanaConfig := params.Config.GetCollectionConfiguration().GetHanaMetricsConfig()

			instance.HanaDbUser, instance.HanaDbPassword, err = sapdiscovery.ReadHANACredentials(ctx, projectID, hanaConfig, params.GCEService)
			if err != nil {
				log.Logger.Warnw("HANA DB Credentials not set, will not collect HANA DB Query related metrics.", "error", err)
			}
		}
	}
	return params.SAPInstances
}

/*
collectAndSend runs the perpetual collect metrics and send to cloud monitoring workflow.

The collectAndSendOnce workflow is called once every process_metrics_frequency.
The function returns an error if no collectors exist. If any errors
occur during collect or send, they are logged and the workflow continues.

For unit testing, the caller can cancel the context to terminate the workflow.
An exit induced by context cancellation returns the last error seen during
the workflow or nil if no error occurred.
*/
func (p *Properties) collectAndSend(ctx context.Context, bo *cloudmonitoring.BackOffIntervals) error {

	if len(p.Collectors) == 0 {
		return fmt.Errorf("expected non-zero collectors, got: %d", len(p.Collectors))
	}

	cf := p.Config.GetCollectionConfiguration().GetProcessMetricsFrequency()
	collectTicker := time.NewTicker(time.Duration(cf) * time.Second)
	defer collectTicker.Stop()

	heartbeatTicker := p.HeartbeatSpec.CreateTicker()
	defer heartbeatTicker.Stop()

	var lastErr error
	for {
		select {
		case <-ctx.Done():
			log.Logger.Info("Context cancelled, exiting collectAndSend.")
			return lastErr
		case <-heartbeatTicker.C:
			p.HeartbeatSpec.Beat()
		case <-collectTicker.C:
			p.HeartbeatSpec.Beat()
			sent, batchCount, err := p.collectAndSendOnce(ctx, bo)
			if err != nil {
				log.Logger.Errorw("Error sending metrics", "error", err)
				lastErr = err
			}
			log.Logger.Infow("Sent metrics from collectAndSend.", "sent", sent, "batches", batchCount, "sleeping", cf)
		}
	}
}

/*
collectAndSendOnce implements the collect(from all collectors) and send workflow
exactly once.

Calls all the registered collectors (HANA, Cluster, etc.) asynchronously
to collect the metrics. A single call to send() uses synchronous Cloud
Monitoring API calls to send the metrics.

Return values are pass-through from send().
*/
func (p *Properties) collectAndSendOnce(ctx context.Context, bo *cloudmonitoring.BackOffIntervals) (sent, batchCount int, err error) {
	var wg sync.WaitGroup
	msgs := make([][]*mrpb.TimeSeries, len(p.Collectors))
	defer (func() { msgs = nil })() // free up reference in memory.
	log.Logger.Debugw("Starting collectors in parallel.", "numberofcollectors", len(p.Collectors))

	for i, collector := range p.Collectors {
		wg.Add(1)
		go func(slot int, c Collector) {
			defer wg.Done()
			msgs[slot] = c.Collect(ctx) // Each collector writes to its own slot.
			log.Logger.Debugw("Collected metrics", "type", c, "numberofmetrics", len(msgs[slot]))
		}(i, collector)
	}
	log.Logger.Debug("Waiting for collectors to finish.")
	wg.Wait()
	return cloudmonitoring.SendTimeSeries(ctx, flatten(msgs), p.Client, bo, p.Config.GetCloudProperties().GetProjectId())
}

// flatten converts an 2D array of metric slices to a flat 1D array of metrics.
func flatten(msgs [][]*mrpb.TimeSeries) []*mrpb.TimeSeries {
	var metrics []*mrpb.TimeSeries
	for _, msg := range msgs {
		metrics = append(metrics, msg...)
	}
	return metrics
}
