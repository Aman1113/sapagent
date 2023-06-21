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

// Package workloadmanager collects workload manager metrics and sends them to Cloud Monitoring
package workloadmanager

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
	monitoringresourcespb "google.golang.org/genproto/googleapis/monitoring/v3"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
	"github.com/zieckey/goini"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2"
	"github.com/GoogleCloudPlatform/sapagent/internal/cloudmonitoring"
	"github.com/GoogleCloudPlatform/sapagent/internal/commandlineexecutor"
	"github.com/GoogleCloudPlatform/sapagent/internal/hanainsights/preprocessor"
	"github.com/GoogleCloudPlatform/sapagent/internal/heartbeat"
	"github.com/GoogleCloudPlatform/sapagent/internal/instanceinfo"
	"github.com/GoogleCloudPlatform/sapagent/internal/log"
	"github.com/GoogleCloudPlatform/sapagent/internal/sapdiscovery"
	"github.com/GoogleCloudPlatform/sapagent/internal/timeseries"
	"github.com/GoogleCloudPlatform/sapagent/internal/usagemetrics"
	cnfpb "github.com/GoogleCloudPlatform/sapagent/protos/configuration"
	rpb "github.com/GoogleCloudPlatform/sapagent/protos/hanainsights/rule"
	sapb "github.com/GoogleCloudPlatform/sapagent/protos/sapapp"
	wlmpb "github.com/GoogleCloudPlatform/sapagent/protos/wlmvalidation"
)

/*
ConfigFileReader abstracts loading and reading files into an io.ReadCloser object. ConfigFileReader Example usage:

	ConfigFileReader(func(path string) (io.ReadCloser, error) {
			file, err := os.Open(path)
			var f io.ReadCloser = file
			return f, err
		})
*/
type ConfigFileReader func(string) (io.ReadCloser, error)

/*
OSStatReader abstracts os.FileInfo reading. OSStatReader Example usage:

	OSStatReader(func(f string) (os.FileInfo, error) {
		return os.Stat(f)
	})
*/
type OSStatReader func(string) (os.FileInfo, error)

// DefaultTokenGetter obtains a "default" oauth2 token source within the getDefaultBearerToken function.
type DefaultTokenGetter func(context.Context, ...string) (oauth2.TokenSource, error)

// JSONCredentialsGetter obtains a JSON oauth2 google credentials within the getJSONBearerToken function.
type JSONCredentialsGetter func(context.Context, []byte, ...string) (*google.Credentials, error)

/*
WorkloadMetrics is a container for monitoring TimeSeries metrics.
*/
type WorkloadMetrics struct {
	Metrics []*monitoringresourcespb.TimeSeries
}

// metricEmitter is a container for constructing metrics from an override configuration file
type metricEmitter struct {
	scanner       *bufio.Scanner
	tmpMetricName string
}

type gceInterface interface {
	GetSecret(ctx context.Context, projectID, secretName string) (string, error)
}

/*
Parameters holds the parameters for all of the Collect* function calls.
*/
type Parameters struct {
	Config                *cnfpb.Configuration
	WorkloadConfig        *wlmpb.WorkloadValidation
	Remote                bool
	ConfigFileReader      ConfigFileReader
	OSStatReader          OSStatReader
	Execute               commandlineexecutor.Execute
	Exists                commandlineexecutor.Exists
	InstanceInfoReader    instanceinfo.Reader
	TimeSeriesCreator     cloudmonitoring.TimeSeriesCreator
	DefaultTokenGetter    DefaultTokenGetter
	JSONCredentialsGetter JSONCredentialsGetter
	OSType                string
	BackOffs              *cloudmonitoring.BackOffIntervals
	HeartbeatSpec         *heartbeat.Spec
	InterfaceAddrsGetter  InterfaceAddrsGetter
	OSReleaseFilePath     string
	netweaverPresent      float64
	GCEService            gceInterface
	HANAInsightRules      []*rpb.Rule
	// fields derived from parsing the file specified by OSReleaseFilePath
	osVendorID string
	osVersion  string
}

// SetOSReleaseInfo parses the OS release file and sets the values for the
// osVendorID and osVersion fields in the Parameters struct.
func (p *Parameters) SetOSReleaseInfo() {
	if p.ConfigFileReader == nil || p.OSReleaseFilePath == "" {
		log.Logger.Debug("A ConfigFileReader and OSReleaseFilePath must be set.")
		return
	}

	file, err := p.ConfigFileReader(p.OSReleaseFilePath)
	if err != nil {
		log.Logger.Warnw(fmt.Sprintf("Could not read from %s", p.OSReleaseFilePath), "error", err)
		return
	}
	defer file.Close()

	ini := goini.New()
	if err := ini.ParseFrom(file, "\n", "="); err != nil {
		log.Logger.Warnw(fmt.Sprintf("Failed to parse from %s", p.OSReleaseFilePath), "error", err)
		return
	}

	id, ok := ini.Get("ID")
	if !ok {
		log.Logger.Warn(fmt.Sprintf("Could not read ID from %s", p.OSReleaseFilePath))
		id = ""
	}
	p.osVendorID = strings.ReplaceAll(strings.TrimSpace(id), `"`, "")

	version, ok := ini.Get("VERSION")
	if !ok {
		log.Logger.Warn(fmt.Sprintf("Could not read VERSION from %s", p.OSReleaseFilePath))
		version = ""
	}
	if vf := strings.Fields(version); len(vf) > 0 {
		p.osVersion = strings.ReplaceAll(strings.TrimSpace(vf[0]), `"`, "")
	}
}

var (
	now = currentTime
)

const metricOverridePath = "/etc/google-cloud-sap-agent/wlmmetricoverride.yaml"
const metricTypePrefix = "workload.googleapis.com/sap/validation/"

func currentTime() int64 {
	return time.Now().Unix()
}

func start(ctx context.Context, params Parameters) {
	log.Logger.Info("Starting collection of Workload Manager metrics")
	cmf := time.Duration(params.Config.GetCollectionConfiguration().GetWorkloadValidationMetricsFrequency()) * time.Second
	if cmf <= 0 {
		// default it to 5 minutes
		cmf = time.Duration(5) * time.Minute
	}
	configurableMetricsTicker := time.NewTicker(cmf)
	defer configurableMetricsTicker.Stop()

	dbmf := time.Duration(params.Config.GetCollectionConfiguration().GetWorkloadValidationDbMetricsFrequency()) * time.Second
	if dbmf <= 0 {
		// default it to 1 hour
		dbmf = time.Duration(3600) * time.Second
	}
	databaseMetricTicker := time.NewTicker(dbmf)
	defer databaseMetricTicker.Stop()

	heartbeatTicker := params.HeartbeatSpec.CreateTicker()
	defer heartbeatTicker.Stop()

	// Do not wait for the first tick and start metric collection immediately.
	select {
	case <-ctx.Done():
		log.Logger.Debug("cancellation requested")
		return
	default:
		collectWorkloadMetricsOnce(ctx, params)
		collectDBMetricsOnce(ctx, params)
	}

	for {
		select {
		case <-ctx.Done():
			log.Logger.Debug("cancellation requested")
			return
		case <-heartbeatTicker.C:
			params.HeartbeatSpec.Beat()
		case <-configurableMetricsTicker.C:
			collectWorkloadMetricsOnce(ctx, params)
		case <-databaseMetricTicker.C:
			collectDBMetricsOnce(ctx, params)
		}
	}
}

// collectWorkloadMetricsOnce issues a heartbeat and initiates one round of metric collection.
func collectWorkloadMetricsOnce(ctx context.Context, params Parameters) {
	params.HeartbeatSpec.Beat()
	if params.Remote {
		log.Logger.Info("Collecting metrics from remote instances")
		collectAndSendRemoteMetrics(ctx, params)
		return
	}
	log.Logger.Info("Collecting metrics from this instance")
	metrics := collectMetricsFromConfig(ctx, params, metricOverridePath)
	sendMetrics(ctx, metrics, params.Config.GetCloudProperties().GetProjectId(), &params.TimeSeriesCreator, params.BackOffs)
}

// StartMetricsCollection continuously collects Workload Manager metrics for SAP workloads.
// Returns true if the collection goroutine is started, and false otherwise.
func StartMetricsCollection(ctx context.Context, params Parameters) bool {
	if (params.Config.GetCollectionConfiguration() == nil || params.Config.GetCollectionConfiguration().GetWorkloadValidationRemoteCollection() == nil) &&
		!params.Config.GetCollectionConfiguration().GetCollectWorkloadValidationMetrics() {
		log.Logger.Info("Not collecting Workload Manager metrics")
		return false
	}
	if params.OSType == "windows" {
		log.Logger.Info("Workload Manager metrics collection is not supported for windows platform.")
		return false
	}
	go usagemetrics.LogActionDaily(usagemetrics.CollectWLMMetrics)
	go start(ctx, params)
	return true
}

// collectMetricsFromConfig returns the result of metric collection using the
// collection definition configuration supplied to the agent.
//
// The results of this function can be overridden using a metricOverride file.
func collectMetricsFromConfig(ctx context.Context, params Parameters, metricOverride string) WorkloadMetrics {
	log.Logger.Info("Collecting Workload Manager metrics...")
	if fileInfo, err := params.OSStatReader(metricOverride); fileInfo != nil && err == nil {
		log.Logger.Info("Using override metrics from yaml file")
		return collectOverrideMetrics(params.Config, params.ConfigFileReader, metricOverride)
	}

	// Read the latest instance info for this system.
	params.InstanceInfoReader.Read(ctx, params.Config, instanceinfo.NetworkInterfaceAddressMap)

	// Collect all metrics specified by the WLM Validation config.
	var system, corosync, hana, netweaver, pacemaker, custom WorkloadMetrics
	var wg sync.WaitGroup
	wg.Add(5)
	go func() {
		defer wg.Done()
		system = CollectSystemMetricsFromConfig(ctx, params)
	}()
	go func() {
		defer wg.Done()
		hana = CollectHANAMetricsFromConfig(ctx, params)
	}()
	go func() {
		defer wg.Done()
		netweaver = CollectNetWeaverMetricsFromConfig(ctx, params)
	}()
	go func() {
		defer wg.Done()
		pacemaker = CollectPacemakerMetricsFromConfig(ctx, params)
		v := 0.0
		if len(pacemaker.Metrics) > 0 && len(pacemaker.Metrics[0].Points) > 0 {
			v = pacemaker.Metrics[0].Points[0].GetValue().GetDoubleValue()
		}
		corosync = CollectCorosyncMetricsFromConfig(ctx, params, v)
	}()
	go func() {
		defer wg.Done()
		custom = CollectCustomMetricsFromConfig(ctx, params)
	}()
	wg.Wait()

	// Append the system metrics to all other metrics.
	systemLabels := system.Metrics[0].Metric.Labels
	appendLabels(corosync.Metrics[0].Metric.Labels, systemLabels)
	appendLabels(hana.Metrics[0].Metric.Labels, systemLabels)
	appendLabels(netweaver.Metrics[0].Metric.Labels, systemLabels)
	appendLabels(pacemaker.Metrics[0].Metric.Labels, systemLabels)
	appendLabels(custom.Metrics[0].Metric.Labels, systemLabels)

	// Concatenate all of the metrics together.
	allMetrics := []*monitoringresourcespb.TimeSeries{}
	allMetrics = append(allMetrics, system.Metrics...)
	allMetrics = append(allMetrics, corosync.Metrics...)
	allMetrics = append(allMetrics, hana.Metrics...)
	allMetrics = append(allMetrics, netweaver.Metrics...)
	allMetrics = append(allMetrics, pacemaker.Metrics...)
	allMetrics = append(allMetrics, custom.Metrics...)

	return WorkloadMetrics{Metrics: allMetrics}
}

func appendLabels(dst map[string]string, src map[string]string) {
	for k, v := range src {
		dst[k] = v
	}
}

/*
Utilize an override configuration file to collect metrics for testing purposes. This allows
sending WLM metrics without creating specific SAP setups. The override file will contain the metric
type followed by all metric labels associated with that type. Example override file contents:

metric: system

	  metric_value: 1
		os: rhel-8.4
		agent_version: 2.6
		gcloud: true

metric: hana

	metric_value: 0
	disk_log_mount: /hana/log
*/
func collectOverrideMetrics(config *cnfpb.Configuration, reader ConfigFileReader, metricOverride string) WorkloadMetrics {
	file, err := reader(metricOverride)
	if err != nil {
		log.Logger.Warnw("Could not read the metric override file", "error", err)
		return WorkloadMetrics{}
	}
	defer file.Close()

	wm := WorkloadMetrics{}
	scanner := bufio.NewScanner(file)
	metricEmitter := metricEmitter{scanner, ""}
	for {
		metricName, metricValue, labels, last := metricEmitter.getMetric()
		if metricName != "" {
			wm.Metrics = append(wm.Metrics, createTimeSeries(metricTypePrefix+metricName, labels, metricValue, config)...)
		}
		if last {
			break
		}
	}
	return wm
}

// Reads all metric labels for a type. Returns false if there is more content in the scanner, true otherwise.
func (e *metricEmitter) getMetric() (string, float64, map[string]string, bool) {
	metricName := e.tmpMetricName
	metricValue := 0.0
	labels := make(map[string]string)
	var err error
	for e.scanner.Scan() {
		key, value, found := parseScannedText(e.scanner.Text())
		if !found {
			continue
		}
		switch key {
		case "metric":
			if metricName != "" {
				e.tmpMetricName = value
				return metricName, metricValue, labels, false
			}
			metricName = value
		case "metric_value":
			if metricValue, err = strconv.ParseFloat(value, 64); err != nil {
				log.Logger.Warnw("Failed to parse float", "value", value, "error", err)
			}
		default:
			labels[key] = strings.TrimSpace(value)
		}
	}
	if err = e.scanner.Err(); err != nil {
		log.Logger.Warnw("Could not read from the override metrics file", "error", err)
	}
	return metricName, metricValue, labels, true
}

// parseScannedText extracts a key and value pair from a scanned line of text.
//
// The expected format for the text string is: '<key>: <value>'.
func parseScannedText(text string) (key, value string, found bool) {
	// Ignore empty lines and comments.
	if text == "" || strings.HasPrefix(text, "#") {
		return "", "", false
	}
	key, value, found = strings.Cut(text, ":")
	if !found {
		log.Logger.Warnw("Could not parse key, value pair. Expected format: '<key>: <value>'", "text", text)
	}
	return strings.TrimSpace(key), strings.TrimSpace(value), found
}

func sendMetrics(ctx context.Context, wm WorkloadMetrics, p string, mc *cloudmonitoring.TimeSeriesCreator, bo *cloudmonitoring.BackOffIntervals) int {
	if wm.Metrics == nil || len(wm.Metrics) == 0 {
		log.Logger.Info("No metrics to send to Cloud Monitoring")
		return 0
	}
	// debugging metric data being sent
	for _, m := range wm.Metrics {
		if len(m.GetPoints()) == 0 {
			log.Logger.Debugw("  Metric has no point data", "metric", m.GetMetric().GetType())
			continue
		}
		log.Logger.Debugw("  Metric", "metric", m.GetMetric().GetType(), "value", m.GetPoints()[0].GetValue().GetDoubleValue())
		for k, v := range m.GetMetric().GetLabels() {
			log.Logger.Debugw("    Label", "key", k, "value", v)
		}
	}
	log.Logger.Infow("Sending metrics to Cloud Monitoring...", "number", len(wm.Metrics))
	request := monitoringpb.CreateTimeSeriesRequest{
		Name:       fmt.Sprintf("projects/%s", p),
		TimeSeries: wm.Metrics}
	if err := cloudmonitoring.CreateTimeSeriesWithRetry(ctx, *mc, &request, bo); err != nil {
		log.Logger.Errorw("Failed to send metrics to Cloud Monitoring", "error", err)
		usagemetrics.Error(usagemetrics.WLMMetricCollectionFailure) // Workload metrics collection failure
		return 0
	}
	log.Logger.Infow("Sent metrics to Cloud Monitoring.", "number", len(wm.Metrics))
	return len(wm.Metrics)
}

func createTimeSeries(t string, l map[string]string, v float64, c *cnfpb.Configuration) []*monitoringresourcespb.TimeSeries {
	now := &timestamppb.Timestamp{
		Seconds: now(),
	}

	p := timeseries.Params{
		BareMetal:    c.BareMetal,
		CloudProp:    c.CloudProperties,
		MetricType:   t,
		MetricLabels: l,
		Timestamp:    now,
		Float64Value: v,
	}
	return []*monitoringresourcespb.TimeSeries{timeseries.BuildFloat64(p)}
}

// DiscoverNetWeaver updates a field in Parameters struct based on presence of a Netweaver instance.
func (p *Parameters) DiscoverNetWeaver(ctx context.Context) {
	log.Logger.Info("Discovering Netweaver instances for Workload Manager Metrics.")
	for _, instance := range sapdiscovery.SAPApplications(ctx).Instances {
		if instance.Type == sapb.InstanceType_NETWEAVER {
			log.Logger.Info("Found Netweaver instance.")
			p.netweaverPresent = 1
			break
		}
	}
}

// ReadHANAInsightsRules reads the HANA Insights rules.
func (p *Parameters) ReadHANAInsightsRules() {
	var err error
	if p.HANAInsightRules, err = preprocessor.ReadRules(preprocessor.RuleFilenames); err != nil {
		log.Logger.Errorw("Error Reading HANA Insights rules", "error", err)
	}
}
