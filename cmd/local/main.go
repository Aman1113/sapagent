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

// Package main serves as the Main entry point for the GC SAP Agent.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	backoff "github.com/cenkalti/backoff/v4"
	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2"
	"github.com/GoogleCloudPlatform/sapagent/internal/agentmetrics"
	"github.com/GoogleCloudPlatform/sapagent/internal/commandlineexecutor"
	"github.com/GoogleCloudPlatform/sapagent/internal/configuration"
	"github.com/GoogleCloudPlatform/sapagent/internal/gce"
	"github.com/GoogleCloudPlatform/sapagent/internal/gce/metadataserver"
	"github.com/GoogleCloudPlatform/sapagent/internal/hostmetrics/agenttime"
	"github.com/GoogleCloudPlatform/sapagent/internal/hostmetrics/cloudmetricreader"
	"github.com/GoogleCloudPlatform/sapagent/internal/hostmetrics"
	"github.com/GoogleCloudPlatform/sapagent/internal/instanceinfo"
	"github.com/GoogleCloudPlatform/sapagent/internal/log"
	"github.com/GoogleCloudPlatform/sapagent/internal/processmetrics/maintenance"
	"github.com/GoogleCloudPlatform/sapagent/internal/processmetrics"
	"github.com/GoogleCloudPlatform/sapagent/internal/system"
	"github.com/GoogleCloudPlatform/sapagent/internal/usagemetrics"
	"github.com/GoogleCloudPlatform/sapagent/internal/workloadmanager"

	cpb "github.com/GoogleCloudPlatform/sap-agent/protos/configuration"
	iipb "github.com/GoogleCloudPlatform/sap-agent/protos/instanceinfo"
)

const usage = `Usage of google-cloud-sap-agent:
  -h, --help prints help information
  -c=PATH, --config=PATH path to configuration.json
	-mm=true|false, --maintenancemode=true|false to configure maintenance mode
	-mm-show, --maintenancemode-show displays the current value configured for maintenancemode
`

var (
	configPath, logfile, usagePriorVersion, usageStatus string
	logUsage                                            bool
	usageAction, usageError                             int
	config                                              *cpb.Configuration
	errQuiet                                            = fmt.Errorf("a quiet error which just signals the program to exit")
	osStatReader                                        = workloadmanager.OSStatReader(func(f string) (os.FileInfo, error) {
		return os.Stat(f)
	})
	configFileReader = workloadmanager.ConfigFileReader(func(path string) (io.ReadCloser, error) {
		file, err := os.Open(path)
		var f io.ReadCloser = file
		return f, err
	})
	commandRunnerNoSpace = commandlineexecutor.CommandRunnerNoSpace(func(exe string, args ...string) (string, string, error) {
		return commandlineexecutor.ExecuteCommand(exe, args...)
	})
	commandRunner = commandlineexecutor.CommandRunner(func(exe string, args string) (string, string, error) {
		return commandlineexecutor.ExpandAndExecuteCommand(exe, args)
	})
	commandExistsRunner = commandlineexecutor.CommandExistsRunner(func(exe string) bool {
		return commandlineexecutor.CommandExists(exe)
	})
	defaultTokenGetter = workloadmanager.DefaultTokenGetter(func(ctx context.Context, scopes ...string) (oauth2.TokenSource, error) {
		return google.DefaultTokenSource(ctx, scopes...)
	})
	jsonCredentialsGetter = workloadmanager.JSONCredentialsGetter(func(ctx context.Context, json []byte, scopes ...string) (*google.Credentials, error) {
		return google.CredentialsFromJSON(ctx, json, scopes...)
	})
)

func fetchCloudProperties() *iipb.CloudProperties {
	exp := backoff.NewExponentialBackOff()
	exp.InitialInterval = 5 * time.Second
	bo := backoff.WithMaxRetries(exp, 2) // 2 retries (3 total attempts)
	return metadataserver.CloudPropertiesWithRetry(bo)
}

func configureUsageMetrics(cp *iipb.CloudProperties, version string) {
	if usagePriorVersion == "" {
		version = configuration.AgentVersion
	}
	usagemetrics.SetAgentProperties(&cpb.AgentProperties{
		Name:            configuration.AgentName,
		Version:         version,
		LogUsageMetrics: true,
	})
	usagemetrics.SetCloudProperties(cp)
}

func setupFlagsAndParse(fs *flag.FlagSet, args []string, fr maintenance.FileReader, fw maintenance.FileWriter) error {
	var help, mntmode, showMntMode bool
	fs.StringVar(&configPath, "config", "", "configuration path")
	fs.StringVar(&configPath, "c", "", "configuration path")
	fs.BoolVar(&help, "help", false, "display help")
	fs.BoolVar(&help, "h", false, "display help")
	fs.BoolVar(&logUsage, "log-usage", false, "invoke usage status logging")
	fs.BoolVar(&logUsage, "lu", false, "invoke usage status logging")
	fs.StringVar(&usagePriorVersion, "log-usage-prior-version", "", "prior installed version")
	fs.StringVar(&usagePriorVersion, "lup", "", "prior installed version")
	fs.StringVar(&usageStatus, "log-usage-status", "", "usage status value")
	fs.StringVar(&usageStatus, "lus", "", "usage status value")
	fs.IntVar(&usageAction, "log-usage-action", 0, "usage action code")
	fs.IntVar(&usageAction, "lua", 0, "usage action code")
	fs.IntVar(&usageError, "log-usage-error", 0, "usage error code")
	fs.IntVar(&usageError, "lue", 0, "usage error code")
	fs.BoolVar(&mntmode, "maintenancemode", false, "configure maintenance mode")
	fs.BoolVar(&mntmode, "mm", false, "configure maintenance mode")
	fs.BoolVar(&showMntMode, "mm-show", false, "show maintenance mode")
	fs.BoolVar(&showMntMode, "maintenancemode-show", false, "show maintenance mode")
	fs.Usage = func() { fmt.Print(usage) }
	fs.Parse(args[1:])

	if isFlagPresent(fs, "mm") || isFlagPresent(fs, "maintenancemode") {
		err := maintenance.UpdateMaintenanceMode(mntmode, fw)
		if err != nil {
			return err
		}
		return errQuiet
	}

	if showMntMode {
		res, err := maintenance.ReadMaintenanceMode(fr)
		if err != nil {
			return err
		}
		log.Print(fmt.Sprintf("Maintenance mode flag for process metrics is set to: %v", res))
		return errQuiet
	}

	if help {
		log.Print(usage)
		return errQuiet
	}

	if logUsage {
		switch {
		case usageStatus == "":
			log.Print("A usage status value is required to be set")
			return errQuiet
		case usageStatus == string(usagemetrics.StatusUpdated) && usagePriorVersion == "":
			log.Print("Prior agent version is required to be set")
			return errQuiet
		case usageStatus == string(usagemetrics.StatusError) && !isFlagPresent(fs, "log-usage-error") && !isFlagPresent(fs, "lue"):
			log.Print("For status ERROR, an error code is required to be set")
			return errQuiet
		case usageStatus == string(usagemetrics.StatusAction) && !isFlagPresent(fs, "log-usage-action") && !isFlagPresent(fs, "lua"):
			log.Print("For status ACTION, an action code is required to be set")
			return errQuiet
		}
	}

	return nil
}

// isFlagPresent checks if a value for the flag `name` is passed from the
// command line. flag.Lookup did not work in case of bool flags as a missing
// bool flag is treated as false.
func isFlagPresent(fs *flag.FlagSet, name string) bool {
	present := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			present = true
		}
	})
	return present
}

// logUsageStatus makes a call to the appropriate usage metrics API.
func logUsageStatus(status string, actionID, errorID int) error {
	switch usagemetrics.Status(status) {
	case usagemetrics.StatusRunning:
		usagemetrics.Running()
	case usagemetrics.StatusStarted:
		usagemetrics.Started()
	case usagemetrics.StatusStopped:
		usagemetrics.Stopped()
	case usagemetrics.StatusConfigured:
		usagemetrics.Configured()
	case usagemetrics.StatusMisconfigured:
		usagemetrics.Misconfigured()
	case usagemetrics.StatusError:
		usagemetrics.Error(errorID)
	case usagemetrics.StatusInstalled:
		usagemetrics.Installed()
	case usagemetrics.StatusUpdated:
		usagemetrics.Updated(configuration.AgentVersion)
	case usagemetrics.StatusUninstalled:
		usagemetrics.Uninstalled()
	case usagemetrics.StatusAction:
		usagemetrics.Action(actionID)
	default:
		return fmt.Errorf("logUsageStatus() called with an unknown status: %s", status)
	}
	return nil
}

func startServices(goos string) {
	if config.GetCloudProperties() == nil {
		log.Logger.Error("Cloud properties are not set, cannot start services.")
		usagemetrics.Error(1) // Invalid configuration
		return
	}

	shutdownch := make(chan os.Signal, 1)
	signal.Notify(shutdownch, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

	ctx := context.Background()
	gceService, err := gce.New(ctx)
	if err != nil {
		log.Logger.Error("Failed to create GCE service", log.Error(err))
		usagemetrics.Error(3) // Unexpected error
		return
	}
	ppr := &instanceinfo.PhysicalPathReader{goos}
	instanceInfoReader := instanceinfo.New(ppr, gceService)

	// NOTE for wlm remote collection: gcloud scp use --compress
	// binary being sent should have +w set on it so it can overrided on scp
	// should only send the binary to the host if it hasn't been sent during this runtime, maybe?
	//    what about if a host has changed while the remote collection has been running
	//    maybe we send always send it every X minutes / hours
	if config.GetCollectionConfiguration() != nil && config.GetCollectionConfiguration().GetWorkloadValidationRemoteCollection() != nil {
		// When set to collect workload manager metrics remotely then that is all this runtime will do.
		log.Logger.Info("Collecting Workload Manager metrics remotely, will not start any other services")
		wlmparameters := workloadmanager.Parameters{
			Config:               config,
			Remote:               true,
			ConfigFileReader:     configFileReader,
			CommandRunner:        commandRunner,
			CommandRunnerNoSpace: commandRunnerNoSpace,
			InstanceInfoReader:   *instanceInfoReader,
			OSStatReader:         osStatReader,
		}
		workloadmanager.StartMetricsCollection(ctx, wlmparameters)
	} else {
		/* The functions being called here should be asynchronous.
		A typical StartXXX() will do the necessary initialisation synchronously and start its own goroutines
		for the long running tasks. The control should be returned to main immediately after init succeeds.
		*/

		// Start the SAP Host Metrics provider
		mqc, err := monitoring.NewQueryClient(ctx)
		if err != nil {
			log.Logger.Error("Failed to create Cloud Monitoring query client", log.Error(err))
			usagemetrics.Error(3) // Unexpected error
			return
		}
		cmr := &cloudmetricreader.CloudMetricReader{QueryClient: &cloudmetricreader.QueryClient{Client: mqc}}
		at := agenttime.New(agenttime.Clock{})
		hmparams := hostmetrics.Parameters{
			Config:             config,
			InstanceInfoReader: *instanceInfoReader,
			CloudMetricReader:  *cmr,
			AgentTime:          *at,
		}
		hostmetrics.StartSAPHostAgentProvider(ctx, hmparams)

		// Start the Workload Manager metrics collection
		mc, err := monitoring.NewMetricClient(ctx)
		if err != nil {
			log.Logger.Error("Failed to create Cloud Monitoring metric client", log.Error(err))
			usagemetrics.Error(3) // Unexpected error
			return
		}
		wlmparams := workloadmanager.Parameters{
			Config:                config,
			Remote:                false,
			ConfigFileReader:      configFileReader,
			CommandRunner:         commandRunner,
			CommandRunnerNoSpace:  commandRunnerNoSpace,
			CommandExistsRunner:   commandExistsRunner,
			InstanceInfoReader:    *instanceInfoReader,
			OSStatReader:          osStatReader,
			TimeSeriesCreator:     mc,
			DefaultTokenGetter:    defaultTokenGetter,
			JSONCredentialsGetter: jsonCredentialsGetter,
			OSType:                goos,
		}
		workloadmanager.StartMetricsCollection(ctx, wlmparams)

		// Start the Process metrics collection
		pmparams := processmetrics.Parameters{
			Config:       config,
			OSType:       goos,
			MetricClient: processmetrics.NewMetricClient,
		}
		processmetrics.Start(ctx, pmparams)

		system.StartSAPSystemDiscovery(ctx, config)

		agentMetricsParams := agentmetrics.Parameters{
			Config: config,
		}
		agentmetricsService, err := agentmetrics.NewService(ctx, agentMetricsParams)
		if err != nil {
			log.Logger.Error("Failed to create agent metrics service", log.Error(err))
		} else {
			agentmetricsService.Start(ctx)
		}
	}

	/* (TODO: b/246271815): Add service status monitoring and restart capabilities in main.go.
	 Sleep forever by blocking current goroutine. Other goroutines will continue to run. Once the
		to-do is complete, this will be responsible for monitoring the status of services and
	 	restarting the failed ones as necessary.
	*/
	go logRunningDaily()

	// wait for the shutdown signal
	<-shutdownch
	// once we have a shutdown event we will wait for up to 3 seconds before for final terminations
	_, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer handleShutdown(cancel)
}

// logRunningDaily log that the agent is running once a day.
func logRunningDaily() {
	for {
		usagemetrics.Running()
		// sleep for 24 hours and a minute, we only log running once a day
		time.Sleep(24*time.Hour + 1*time.Minute)
	}
}

func handleShutdown(cancel context.CancelFunc) {
	log.Logger.Info("Shutting down...")
	usagemetrics.Stopped()
	cancel()
}

func main() {
	fs := flag.NewFlagSet("cli-flags", flag.ExitOnError)
	err := setupFlagsAndParse(fs, os.Args, maintenance.ModeReader{}, maintenance.ModeWriter{})
	switch {
	case errors.Is(err, errQuiet):
		os.Exit(0)
	case err != nil:
		log.Print(err.Error())
		os.Exit(1)
	}
	config = configuration.ReadFromFile(configPath, os.ReadFile)
	log.SetupLoggingToFile(runtime.GOOS, config.GetLogLevel())
	cloudProps := fetchCloudProperties()
	config = configuration.ApplyDefaults(config, cloudProps)
	configureUsageMetrics(cloudProps, usagePriorVersion)
	if logUsage {
		err = logUsageStatus(usageStatus, usageAction, usageError)
		if err != nil {
			log.Logger.Warn("Could not log usage", log.Error(err))
		}
		// exit the pgoram, this was a one time execution to just log a usage
		os.Exit(0)
	}

	usagemetrics.Configured()
	usagemetrics.Started()
	startServices(runtime.GOOS)
}
