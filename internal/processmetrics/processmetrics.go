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
	"github.com/GoogleCloudPlatform/sapagent/internal/log"
	"github.com/GoogleCloudPlatform/sapagent/internal/processmetrics/cluster"
	"github.com/GoogleCloudPlatform/sapagent/internal/processmetrics/computeresources"
	"github.com/GoogleCloudPlatform/sapagent/internal/processmetrics/hana"
	"github.com/GoogleCloudPlatform/sapagent/internal/processmetrics/maintenance"
	"github.com/GoogleCloudPlatform/sapagent/internal/processmetrics/netweaver"
	"github.com/GoogleCloudPlatform/sapagent/internal/processmetrics/sapdiscovery"
	"github.com/GoogleCloudPlatform/sapagent/internal/processmetrics/sapservice"
	"github.com/GoogleCloudPlatform/sapagent/internal/usagemetrics"

	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
	mrpb "google.golang.org/genproto/googleapis/monitoring/v3"
	cpb "github.com/GoogleCloudPlatform/sap-agent/protos/configuration"
	sapb "github.com/GoogleCloudPlatform/sap-agent/protos/sapapp"
)

type (
	// Collector interface is SAP application specific metric collection logic.
	// This needs to be implennted by application specific modules that want to leverage
	// startMetricGroup functionality.
	Collector interface {
		Collect() []*sapdiscovery.Metrics
	}

	// Properties has necessary context for Metrics collection.
	Properties struct {
		SAPInstances *sapb.SAPInstances // Optional for production use cases, used by unit tests.
		Config       *cpb.Configuration
		Client       cloudmonitoring.TimeSeriesCreator
		Collectors   []Collector
	}

	// CreateMetricClient provides an easily testable translation to the cloud monitoring API.
	CreateMetricClient func(ctx context.Context) (cloudmonitoring.TimeSeriesCreator, error)

	// Parameters has parameters necessary to invoke Start().
	Parameters struct {
		Config       *cpb.Configuration
		OSType       string
		MetricClient CreateMetricClient
		SAPInstances *sapb.SAPInstances
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

	log.Logger.Infof("Configuration option collect_process_metrics set to: %t.", cpm)

	switch {
	case !cpm:
		log.Logger.Info("Not collecting Process Metrics.")
		return false
	case parameters.OSType == "windows":
		log.Logger.Info("Process Metrics collection is not supported for windows platform.")
		return false
	case cf < minimumFrequency:
		log.Logger.Infof("Process metrics frequency: %d is smaller than minimum supported value: %d.", cf, minimumFrequency)
		log.Logger.Info("Not collecting Process Metrics.")
		usagemetrics.Error(3) // Unexpected error
		return false
	}

	mc, err := parameters.MetricClient(ctx)
	if err != nil {
		log.Logger.Error("Failed to create Cloud Monitoring client", log.Error(err))
		usagemetrics.Error(3) // Unexpected error
		return false
	}

	sapInstances := instancesWithCredentials(ctx, &parameters)
	if len(sapInstances.GetInstances()) == 0 {
		log.Logger.Error("No SAP Instances found. Cannot start process metrics collection.")
		usagemetrics.Error(3) // Unexpected error
		return false
	}

	log.Logger.Info("Starting process metrics collection in background.")
	usagemetrics.Action(3) // Collecting process metrics
	p := create(parameters.Config, mc, sapInstances)
	// NOMUTANTS--will be covered by integration testing
	go p.collectAndSend(ctx)
	return true
}

// NewMetricClient is the production version that calls cloud monitoring API.
func NewMetricClient(ctx context.Context) (cloudmonitoring.TimeSeriesCreator, error) {
	return monitoring.NewMetricClient(ctx)
}

// create sets up the processmetrics properties and metric collectors for SAP Instances.
func create(config *cpb.Configuration, client cloudmonitoring.TimeSeriesCreator, sapInstances *sapb.SAPInstances) *Properties {
	p := &Properties{
		SAPInstances: sapInstances,
		Config:       config,
		Client:       client,
	}

	log.Logger.Info("Creating maintenance mode collector.")
	maintenanceModeCollector := &maintenance.InstanceProperties{
		Config: p.Config,
		Client: p.Client,
		Reader: maintenance.ModeReader{},
	}

	log.Logger.Info("Creating SAP additional metrics collector for sapservices (active and enabled metric).")
	sapServiceCollector := &sapservice.InstanceProperties{
		Config:   p.Config,
		Client:   p.Client,
		Executor: commandlineexecutor.ExpandAndExecuteCommand,
	}

	log.Logger.Info("Creating SAP control processes per process CPU, memory usage metrics collector.")
	sapStartCollector := &computeresources.SAPControlProcInstanceProperties{
		Config:     p.Config,
		Client:     p.Client,
		Executor:   commandlineexecutor.ExpandAndExecuteCommand,
		FileReader: maintenance.ModeReader{},
	}

	p.Collectors = append(p.Collectors, maintenanceModeCollector, sapServiceCollector, sapStartCollector)

	for _, instance := range p.SAPInstances.GetInstances() {
		if p.SAPInstances.GetLinuxClusterMember() {
			log.Logger.Infof("Creating cluster collector for instance %q.", instance.GetInstanceId())
			clusterCollector := &cluster.InstanceProperties{
				SAPInstance: instance,
				Config:      p.Config,
				Client:      p.Client,
			}
			p.Collectors = append(p.Collectors, clusterCollector)
		}
		if instance.GetType() == sapb.InstanceType_HANA {
			log.Logger.Infof("Creating HANA per process CPU, memory usage metrics collector for instance %q.", instance.GetInstanceId())
			hanaComputeresourcesCollector := &computeresources.HanaInstanceProperties{
				Config:      p.Config,
				Client:      p.Client,
				Executor:    commandlineexecutor.ExpandAndExecuteCommand,
				FileReader:  maintenance.ModeReader{},
				SAPInstance: instance,
				Runner: &commandlineexecutor.Runner{
					User:       instance.GetUser(),
					Executable: instance.GetSapcontrolPath(),
					Args:       fmt.Sprintf("-nr %s -function GetProcessList -format script", instance.GetInstanceNumber()),
					Env:        []string{"LD_LIBRARY_PATH=" + instance.GetLdLibraryPath()},
				},
			}

			log.Logger.Infof("Creating HANA collector for instance %q.", instance.GetInstanceId())
			hanaCollector := &hana.InstanceProperties{
				SAPInstance: instance,
				Config:      p.Config,
				Client:      p.Client,
			}
			p.Collectors = append(p.Collectors, hanaComputeresourcesCollector, hanaCollector)
		}
		if instance.GetType() == sapb.InstanceType_NETWEAVER {
			log.Logger.Info("Creating Netweaver per process CPU, memory usage metrics collector for instance %q.", instance.GetInstanceId())
			netweaverComputeresourcesCollector := &computeresources.NetweaverInstanceProperties{
				Config:      p.Config,
				Client:      p.Client,
				Executor:    commandlineexecutor.ExpandAndExecuteCommand,
				FileReader:  maintenance.ModeReader{},
				SAPInstance: instance,
				Runner: &commandlineexecutor.Runner{
					User:       instance.GetUser(),
					Executable: instance.GetSapcontrolPath(),
					Args:       fmt.Sprintf("-nr %s -function GetProcessList -format script", instance.GetInstanceNumber()),
					Env:        []string{"LD_LIBRARY_PATH=" + instance.GetLdLibraryPath()},
				},
			}

			log.Logger.Infof("Creating Netweaver collector for instance %q.", instance.GetInstanceId())
			netweaverCollector := &netweaver.InstanceProperties{
				SAPInstance: instance,
				Config:      p.Config,
				Client:      p.Client,
			}
			p.Collectors = append(p.Collectors, netweaverComputeresourcesCollector, netweaverCollector)
		}
	}

	log.Logger.Infof("Created %d collectors.", len(p.Collectors))
	return p
}

// instancesWithCredentials run SAP discovery to detect SAP insatances on this machine and update
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

			instance.HanaDbUser, instance.HanaDbPassword, err = sapdiscovery.ReadHANACredentials(ctx, projectID, hanaConfig)
			if err != nil {
				log.Logger.Warn("HANA DB Credentials not set, will not collect HANA DB Query related metrics.", log.Error(err))
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
func (p *Properties) collectAndSend(ctx context.Context) error {

	if len(p.Collectors) == 0 {
		return fmt.Errorf("expected non-zero collectors, got: %d", len(p.Collectors))
	}

	var lastErr error
	cf := p.Config.GetCollectionConfiguration().GetProcessMetricsFrequency()
	time.Sleep(time.Duration(cf) * time.Second)
	for {
		select {
		case <-ctx.Done():
			log.Logger.Info("Context cancelled, exiting collectAndSend.")
			return lastErr
		default:
			sent, batchCount, err := p.collectAndSendOnce(ctx)
			if err != nil {
				log.Logger.Error("Error sending metrics", log.Error(err))
				lastErr = err
			}
			log.Logger.Infof("Sent %d metrics in %d batches, sleeping for %d in collectAndSend.", sent, batchCount, cf)
			time.Sleep(time.Duration(cf) * time.Second)
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
func (p *Properties) collectAndSendOnce(ctx context.Context) (sent, batchCount int, err error) {
	var wg sync.WaitGroup
	msgs := make([][]*sapdiscovery.Metrics, len(p.Collectors))
	log.Logger.Debugf("Start %d collectors in parallel.", len(p.Collectors))

	for i, collector := range p.Collectors {
		wg.Add(1)
		go func(slot int, c Collector) {
			defer wg.Done()
			msgs[slot] = c.Collect() // Each collector writes to its own slot.
			log.Logger.Debugf("Type %T collected %d metrics.", c, len(msgs[slot]))
		}(i, collector)
	}
	log.Logger.Debug("Wait for collectors to finish.")
	wg.Wait()
	return p.send(ctx, flatten(msgs))
}

/*
send sends all the timeseries objects in metrics array to cloud monitoring.

The value maxTSPerRequest is used as upper limit to pack timeseries values per
request. One or more synchrounous API calls are made to write the metrics to cloud
monitoring. If a cloud monitoring API call fails even after retries (done in
cloudmonitoring.go), the remaining measurements are discarded.
  - Returns number of timeseries sent(as sent) to cloud monitoring.
  - Returns the number of batches(as batchCount) for unit testing coverage.
  - Returns error if cloud monitoring API fails.
*/
func (p *Properties) send(ctx context.Context, metrics []*sapdiscovery.Metrics) (sent, batchCount int, err error) {
	var batchTimeSeries []*mrpb.TimeSeries

	for _, m := range metrics {
		batchTimeSeries = append(batchTimeSeries, m.TimeSeries)

		if len(batchTimeSeries) == maxTSPerRequest {
			log.Logger.Debug("Maximum batch size is reached, send the batch.")
			batchCount++
			if err := p.sendBatch(ctx, batchTimeSeries); err != nil {
				return sent, batchCount, err
			}
			sent += len(batchTimeSeries)
			batchTimeSeries = nil
		}
	}
	batchCount++
	if err := p.sendBatch(ctx, batchTimeSeries); err != nil {
		return sent, batchCount, err
	}
	return sent + len(batchTimeSeries), batchCount, nil
}

/*
sendBatch sends one batch of metrics to cloud monitoring using an API call with retries.
Returns an error in case of failures.
*/
func (p *Properties) sendBatch(ctx context.Context, batchTimeSeries []*mrpb.TimeSeries) error {
	log.Logger.Debugf("Sending %d metrics to cloud monitoring.", len(batchTimeSeries))

	req := &monitoringpb.CreateTimeSeriesRequest{
		Name:       fmt.Sprintf("projects/%s", p.Config.GetCloudProperties().GetProjectId()),
		TimeSeries: batchTimeSeries,
	}

	return cloudmonitoring.CreateTimeSeriesWithRetry(ctx, p.Client, req)
}

/*
flatten converts an 2D array of metric slices to a flat 1D array of metrics.
*/
func flatten(msgs [][]*sapdiscovery.Metrics) []*sapdiscovery.Metrics {
	var metrics []*sapdiscovery.Metrics
	for _, msg := range msgs {
		metrics = append(metrics, msg...)
	}
	return metrics
}
