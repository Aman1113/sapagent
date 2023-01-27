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
	"testing"

	"github.com/GoogleCloudPlatform/sapagent/internal/cloudmonitoring/fake"
	cpb "github.com/GoogleCloudPlatform/sapagent/protos/configuration"
	iipb "github.com/GoogleCloudPlatform/sapagent/protos/instanceinfo"
	sapb "github.com/GoogleCloudPlatform/sapagent/protos/sapapp"
)

var (
	defaultConfig = &cpb.Configuration{
		CollectionConfiguration: &cpb.CollectionConfiguration{
			CollectProcessMetrics:       false,
			ProcessMetricsFrequency:     5,
			ProcessMetricsSendFrequency: 60,
		},
		CloudProperties: &iipb.CloudProperties{
			ProjectId:        "test-project",
			InstanceId:       "test-instance",
			Zone:             "test-zone",
			InstanceName:     "test-instance",
			Image:            "test-image",
			NumericProjectId: "123456",
		},
	}
	defaultSAPInstanceHANA = &sapb.SAPInstance{
		Type: sapb.InstanceType_HANA,
	}
	defaultSapControlOutputHANA = `OK
		0 name: hdbdaemon
		0 dispstatus: GREEN
		0 pid: 111
		1 name: hdbcompileserver
		1 dispstatus: GREEN
		1 pid: 222`
)

func (f *fakeRunner) RunWithEnv() (string, string, int, error) {
	return f.stdOut, f.stdErr, f.exitCode, f.err
}

type (
	fakeRunner struct {
		stdOut, stdErr string
		exitCode       int
		err            error
	}
)

func TestCollectForHANA(t *testing.T) {
	tests := []struct {
		name             string
		executor         commandExecutor
		sapControlOutput string
		wantCount        int
	}{
		{
			name: "EmptyPIDsMap",
			executor: func(cmd, args string) (string, string, error) {
				return "", "", nil
			},
			sapControlOutput: "",
			wantCount:        0,
		},
		{
			name: "OnlyMemoryPerProcessMetricAvailable",
			executor: func(cmd, args string) (string, string, error) {
				if cmd == "getconf" {
					return "100\n", "", nil
				}
				return "", "", nil
			},
			sapControlOutput: defaultSapControlOutputHANA,
			wantCount:        3,
		},
		{
			name: "OnlyCPUPerProcessMetricAvailable",
			executor: func(cmd, args string) (string, string, error) {
				if cmd == "getconf" {
					return "100\n", "", nil
				}
				return "", "", nil
			},
			sapControlOutput: `OK
			0 name: msg_server
			0 dispstatus: GREEN
			0 pid: 111
			1 name: enserver
			1 dispstatus: GREEN
			1 pid: 333
			`,
			wantCount: 1,
		},
		{
			name: "FetchedBothMemoryAndCPUMetricsSuccessfully",
			executor: func(cmd, args string) (string, string, error) {
				if cmd == "getconf" {
					return "100\n", "", nil
				}
				return "", "", nil
			},
			sapControlOutput: `OK
			0 name: hdbdaemon
			0 dispstatus: GREEN
			0 pid: 444
			1 name: hdbcompileserver
			1 dispstatus: GREEN
			1 pid: 555
			`,
			wantCount: 8,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testHanaInstanceProps := &HanaInstanceProperties{
				Config:        defaultConfig,
				Client:        &fake.TimeSeriesCreator{},
				Executor:      test.executor,
				SAPInstance:   defaultSAPInstanceHANA,
				Runner:        &fakeRunner{stdOut: test.sapControlOutput},
				NewProcHelper: newProcessWithContextHelperTest,
			}
			got := testHanaInstanceProps.Collect(context.Background())
			if len(got) != test.wantCount {
				t.Errorf("Collect() = %d , want %d", len(got), test.wantCount)
			}

			for _, metric := range got {
				points := metric.TimeSeries.GetPoints()
				if points[0].GetValue().GetDoubleValue() < 0 {
					t.Errorf("Metric value for compute resources cannot be negative.")
				}
			}
		})
	}
}
