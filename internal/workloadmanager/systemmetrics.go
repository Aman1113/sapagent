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

package workloadmanager

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/zieckey/goini"
	"github.com/GoogleCloudPlatform/sapagent/internal/commandlineexecutor"
	"github.com/GoogleCloudPlatform/sapagent/internal/configurablemetrics"
	"github.com/GoogleCloudPlatform/sapagent/internal/log"

	wlmpb "github.com/GoogleCloudPlatform/sapagent/protos/wlmvalidation"
)

var (
	iniParse           = goiniParse
	cmdExists          = commandlineexecutor.CommandExists
	netInterfaceAdddrs = net.InterfaceAddrs
	agentServiceStatus = serviceStatus
	osCaptionExecute   = wmicOsCaptionExecute
	osVersionExecute   = wmicOsVersion
)

// OSReleaseFilePath lists the location of the os-release file in the Linux system.
const OSReleaseFilePath = "/etc/os-release"

// InterfaceAddrsGetter satisfies the function signature for net.InterfaceAddrs().
type InterfaceAddrsGetter func() ([]net.Addr, error)

// CollectSystemMetricsFromConfig collects the system metrics specified by the
// WorkloadValidation config and formats the results as a time series to be
// uploaded to a Collection Storage mechanism.
func CollectSystemMetricsFromConfig(ctx context.Context, params Parameters) WorkloadMetrics {
	log.Logger.Info("Collecting Workload Manager System metrics...")
	t := "workload.googleapis.com/sap/validation/system"
	l := make(map[string]string)

	system := params.WorkloadConfig.GetValidationSystem()
	for _, m := range system.GetSystemMetrics() {
		v := collectSystemVariable(m, params)
		l[m.GetMetricInfo().GetLabel()] = v
	}
	for _, m := range system.GetOsCommandMetrics() {
		k, v := configurablemetrics.CollectOSCommandMetric(ctx, m, params.Execute, params.osVendorID)
		if k != "" {
			l[k] = v
		}
	}

	return WorkloadMetrics{Metrics: createTimeSeries(t, l, 1, params.Config)}
}

// collectSystemVariable collects and returns the metric value for a given system metric variable.
func collectSystemVariable(m *wlmpb.SystemMetric, params Parameters) string {
	v := m.GetValue()
	switch v {
	case wlmpb.SystemVariable_INSTANCE_NAME:
		return params.Config.GetCloudProperties().GetInstanceName()
	case wlmpb.SystemVariable_OS_NAME_VERSION:
		return fmt.Sprintf("%s-%s", params.osVendorID, params.osVersion)
	case wlmpb.SystemVariable_AGENT_NAME:
		return params.Config.GetAgentProperties().GetName()
	case wlmpb.SystemVariable_AGENT_VERSION:
		return params.Config.GetAgentProperties().GetVersion()
	case wlmpb.SystemVariable_NETWORK_IPS:
		return networkIPAddrs(params)
	default:
		log.Logger.Warnw("System metric has no system variable value to collect from", "metric", m.GetMetricInfo().GetLabel())
		return ""
	}
}

// networkIPAddrs parses the network interface addresses from the system.
func networkIPAddrs(params Parameters) string {
	addrs, err := params.InterfaceAddrsGetter()
	if err != nil {
		log.Logger.Warnw("Could not get network interface addresses", "error", err)
		return ""
	}
	v := []string{}
	for _, ipaddr := range addrs {
		v = append(v, ipaddr.String())
	}
	return strings.Join(v, ",")
}

// CollectSystemMetrics will collect the system metrics for Workload Manager and send them to the
// channel wm
//
// This is a legacy collection method that can be removed from the codebase
// once configurable WLM metric collection is fully implemented.
func CollectSystemMetrics(ctx context.Context, params Parameters, wm chan<- WorkloadMetrics) {
	log.Logger.Info("Collecting Workload Manager System metrics...")
	gcl := "false"
	if cmdExists("gcloud") {
		gcl = "true"
	}
	gsu := "false"
	if cmdExists("gsutil") {
		gsu = "true"
	}
	l := buildLabelMap(ctx, params, gcl, gsu)
	t := "workload.googleapis.com/sap/validation/system"
	m := createTimeSeries(t, l, 1, params.Config)
	wm <- WorkloadMetrics{Metrics: m}
}

func netInterfacesValue() string {
	addrs, err := netInterfaceAdddrs()
	if err != nil {
		log.Logger.Warnw("Could not get network interface addresses", "error", err)
		return ""
	}
	v := []string{}
	for _, ipaddr := range addrs {
		v = append(v, ipaddr.String())
	}
	return strings.Join(v, ",")
}

func goiniParse(f string) *goini.INI {
	ini := goini.New()
	err := ini.ParseFile(f)
	if err != nil {
		log.Logger.Warnw("Could not read OS information from /etc/os-release", "error", err)
	}
	return ini
}

func linuxOsRelease() string {
	ini := iniParse("/etc/os-release")
	id, ok := ini.Get("ID")
	if !ok {
		log.Logger.Warn("Could not read ID from /etc/os-release")
		id = ""
	}
	id = strings.TrimSpace(id)
	ver, ok := ini.Get("VERSION")
	if !ok {
		log.Logger.Warn("Could not read VERSION from /etc/os-release")
		ver = ""
	}
	vf := strings.Fields(ver)
	v := ""
	if len(vf) > 0 {
		v = strings.TrimSpace(vf[0])
	}
	return strings.ReplaceAll(id, `"`, "") + "-" + strings.ReplaceAll(v, `"`, "")
}

func wmicOsCaptionExecute(ctx context.Context) commandlineexecutor.Result {
	return commandlineexecutor.ExecuteCommand(ctx, commandlineexecutor.Params{
		Executable: "wmic",
		Args:       []string{"Caption/Format:List"},
	})
}

func wmicOsVersion(ctx context.Context) commandlineexecutor.Result {
	return commandlineexecutor.ExecuteCommand(ctx, commandlineexecutor.Params{
		Executable: "wmic",
		Args:       []string{"Version/Format:List"},
	})
}

// Trims all whitespace, replaces space with underscore, and lowercases the output.
// The function will only return the value portion of the wmic output if there is a key=value format.
// If the input does not contain a key=value format then the entire input will be returned.
// Example input: "Caption=Microsoft Windows Server 2019 Datacenter".
func trimAndSplitWmicOutput(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "_")
	sp := strings.Split(s, "=")
	if len(sp) < 2 {
		return strings.ToLower(s)
	}
	return strings.ToLower(sp[1])
}

func windowsOsRelease(ctx context.Context) string {
	cresult := osCaptionExecute(ctx)
	c := cresult.StdOut
	if cresult.Error != nil {
		log.Logger.Warnw("Could not execute wmic get Caption", "stderr", cresult.StdErr, "error", cresult.Error)
		c = ""
	}
	c = trimAndSplitWmicOutput(c)
	vresult := osVersionExecute(ctx)
	v := vresult.StdOut
	if vresult.Error != nil {
		log.Logger.Warnw("Could not execute wmic get Version", "stderr", vresult.StdErr, "error", vresult.Error)
		v = ""
	}
	v = trimAndSplitWmicOutput(v)
	return c + "-" + v
}

func osRelease(ctx context.Context, runtimeOS string) string {
	if runtimeOS == "windows" {
		return windowsOsRelease(ctx)
	}
	return linuxOsRelease()
}

func serviceStatus(ctx context.Context, runtimeOS string) commandlineexecutor.Result {
	if runtimeOS == "windows" {
		return commandlineexecutor.ExecuteCommand(ctx, commandlineexecutor.Params{
			Executable: "powershell",
			Args:       []string{"-Command", "(Get-Service", "-Name", "google-cloud-sap-agent).Status"},
		})
	}
	return commandlineexecutor.ExecuteCommand(ctx, commandlineexecutor.Params{
		Executable: "systemctl",
		Args:       []string{"is-active", "google-cloud-sap-agent"},
	})
}

func agentState(ctx context.Context, runtimeOS string) string {
	state := "notinstalled"
	result := agentServiceStatus(ctx, runtimeOS)
	if result.Error != nil {
		log.Logger.Warnw("Could not get the agents service status", "error", result.Error)
		return state
	}
	s := strings.TrimSpace(result.StdOut)
	if runtimeOS == "windows" && !strings.Contains(s, "Cannot find any service") {
		state = "notrunning"
		if strings.HasPrefix(s, "Running") {
			state = "running"
		}
	} else if runtimeOS == "linux" && !strings.Contains(s, "could not be found") {
		state = "notrunning"
		if strings.HasPrefix(s, "Active: active") || strings.HasPrefix(s, "active") {
			state = "running"
		}
	}
	return state
}

func buildLabelMap(ctx context.Context, params Parameters, gcl string, gsu string) map[string]string {
	return map[string]string{
		"instance_name": params.Config.GetCloudProperties().GetInstanceName(),
		"os":            osRelease(ctx, params.OSType),
		"agent":         "gcagent",
		"agent_version": params.Config.GetAgentProperties().GetVersion(),
		"agent_state":   agentState(ctx, params.OSType),
		"gcloud":        gcl,
		"gsutil":        gsu,
		"network_ips":   netInterfacesValue(),
	}
}
