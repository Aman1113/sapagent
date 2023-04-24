/*
Copyright 2023 Google LLC

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
	"github.com/GoogleCloudPlatform/sapagent/internal/configurablemetrics"
	"github.com/GoogleCloudPlatform/sapagent/internal/log"
)

// CollectCustomMetricsFromConfig collects any custom metrics as specified by
// the WorkloadValidation config and formats the results as a time series to be
// uploaded to a Collection Storage mechanism.
func CollectCustomMetricsFromConfig(params Parameters) WorkloadMetrics {
	log.Logger.Info("Collecting Workload Manager Custom metrics...")
	t := "workload.googleapis.com/sap/validation/custom"
	l := make(map[string]string)

	custom := params.WorkloadConfig.GetValidationCustom()
	for _, m := range custom.GetOsCommandMetrics() {
		k, v := configurablemetrics.CollectOSCommandMetric(m, params.CommandRunnerNoSpace, params.osVendorID)
		if k != "" {
			l[k] = v
		}
	}

	return WorkloadMetrics{Metrics: createTimeSeries(t, l, 0, params.Config)}
}
