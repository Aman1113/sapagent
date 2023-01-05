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

package processmetrics

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"
	"github.com/GoogleCloudPlatform/sapagent/internal/cloudmonitoring"
	"github.com/GoogleCloudPlatform/sapagent/internal/cloudmonitoring/fake"
	"github.com/GoogleCloudPlatform/sapagent/internal/processmetrics/sapdiscovery"

	mrpb "google.golang.org/genproto/googleapis/monitoring/v3"
	cpb "github.com/GoogleCloudPlatform/sapagent/protos/configuration"
	ipb "github.com/GoogleCloudPlatform/sapagent/protos/instanceinfo"
	sapb "github.com/GoogleCloudPlatform/sapagent/protos/sapapp"
)

var (
	defaultCloudProperties = &ipb.CloudProperties{
		ProjectId:        "test-project",
		InstanceId:       "test-instance",
		Zone:             "test-zone",
		InstanceName:     "test-instance",
		Image:            "test-image",
		NumericProjectId: "123456",
	}

	defaultConfig = &cpb.Configuration{
		CollectionConfiguration: &cpb.CollectionConfiguration{
			CollectProcessMetrics:   true,
			ProcessMetricsFrequency: 5,
		},
		CloudProperties: defaultCloudProperties,
	}

	quickTestConfig = &cpb.Configuration{
		CollectionConfiguration: &cpb.CollectionConfiguration{
			CollectProcessMetrics:   true,
			ProcessMetricsFrequency: 1, //Use small value for quick unit tests.
		},
		CloudProperties: defaultCloudProperties,
	}
)

type (
	fakeProperties struct {
		SAPInstances *sapb.SAPInstances
		Config       *cpb.Configuration
		Client       cloudmonitoring.TimeSeriesCreator
	}

	fakeCollector struct {
		timeSeriesCount int
	}
)

func (f *fakeCollector) Collect() []*sapdiscovery.Metrics {
	m := make([]*sapdiscovery.Metrics, f.timeSeriesCount)
	for i := 0; i < f.timeSeriesCount; i++ {
		m[i] = &sapdiscovery.Metrics{
			TimeSeries: &mrpb.TimeSeries{},
		}
	}
	return m
}

func fakeCollectors(count, timeSerisCountPerCollector int) []Collector {
	collectors := make([]Collector, count)
	for i := 0; i < count; i++ {
		collectors[i] = &fakeCollector{timeSeriesCount: timeSerisCountPerCollector}
	}
	return collectors
}

func fakeNewMetricClient(ctx context.Context) (cloudmonitoring.TimeSeriesCreator, error) {
	return &fake.TimeSeriesCreator{}, nil
}

func fakeNewMetricClientFailure(ctx context.Context) (cloudmonitoring.TimeSeriesCreator, error) {
	return nil, cmpopts.AnyError
}

func fakeSAPInstances(app string) *sapb.SAPInstances {
	switch app {
	case "HANA":
		return &sapb.SAPInstances{
			Instances: []*sapb.SAPInstance{
				&sapb.SAPInstance{
					Type: sapb.InstanceType_HANA,
				},
			},
		}
	case "HANACluster":
		return &sapb.SAPInstances{
			Instances: []*sapb.SAPInstance{
				&sapb.SAPInstance{
					Type: sapb.InstanceType_HANA,
				},
			},
			LinuxClusterMember: true,
		}
	case "NetweaverCluster":
		return &sapb.SAPInstances{
			Instances: []*sapb.SAPInstance{
				&sapb.SAPInstance{
					Type: sapb.InstanceType_NETWEAVER,
				},
			},
			LinuxClusterMember: true,
		}
	default:
		return nil
	}
}

// The goal of these unit tests is to test the interaction of this package with respective collectors.
// This assumes that the collector is tested by its own unit tests.
func TestStart(t *testing.T) {
	tests := []struct {
		name       string
		parameters Parameters
		want       bool
	}{
		{
			name: "SuccessEnabled",
			parameters: Parameters{
				Config:       defaultConfig,
				OSType:       "linux",
				MetricClient: fakeNewMetricClient,
				SAPInstances: fakeSAPInstances("HANA"),
			},
			want: true,
		},
		{
			name: "FailsDisabled",
			parameters: Parameters{
				Config: &cpb.Configuration{
					CollectionConfiguration: &cpb.CollectionConfiguration{
						CollectProcessMetrics: false,
					},
				},
				OSType:       "linux",
				MetricClient: fakeNewMetricClient,
				SAPInstances: fakeSAPInstances("HANA"),
			},
			want: false,
		},
		{
			name: "FailsForWindowsOS",
			parameters: Parameters{
				Config: defaultConfig,
				OSType: "windows",
			},
			want: false,
		},
		{
			name: "InvalidProcessMetricFrequency",
			parameters: Parameters{
				Config:       quickTestConfig,
				OSType:       "linux",
				MetricClient: fakeNewMetricClient,
				SAPInstances: fakeSAPInstances("HANA"),
			},
			want: false,
		},
		{
			name: "CreateMetricClientFailure",
			parameters: Parameters{
				Config:       defaultConfig,
				OSType:       "linux",
				MetricClient: fakeNewMetricClientFailure,
				SAPInstances: fakeSAPInstances("HANA"),
			},
			want: false,
		},
		{
			name: "ZeroSAPApplications",
			parameters: Parameters{
				Config:       defaultConfig,
				OSType:       "linux",
				MetricClient: fakeNewMetricClient,
				SAPInstances: fakeSAPInstances("NOSAP"),
			},
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := Start(context.Background(), test.parameters)
			if got != test.want {
				t.Errorf("Start(%v), got: %t want: %t", test.parameters, got, test.want)
			}
		})
	}
}

func TestCreate(t *testing.T) {
	tests := []struct {
		name               string
		sapInstances       *sapb.SAPInstances
		wantCollectorCount int
	}{
		{
			name:               "HANAStandaloneInstance",
			sapInstances:       fakeSAPInstances("HANA"),
			wantCollectorCount: 5,
		},
		{
			name:               "HANAClusterInstance",
			sapInstances:       fakeSAPInstances("HANACluster"),
			wantCollectorCount: 6,
		},
		{
			name:               "NetweaverClusterInstance",
			sapInstances:       fakeSAPInstances("NetweaverCluster"),
			wantCollectorCount: 6,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := create(defaultConfig, &fake.TimeSeriesCreator{}, test.sapInstances)

			if len(got.Collectors) != test.wantCollectorCount {
				t.Errorf("create() returned %d collectors, want %d", len(got.Collectors), test.wantCollectorCount)
			}
		})
	}
}

func createFakeMetrics(count int) []*sapdiscovery.Metrics {
	var metrics []*sapdiscovery.Metrics

	for i := 0; i < count; i++ {
		metrics = append(metrics, &sapdiscovery.Metrics{
			TimeSeries: &mrpb.TimeSeries{},
		})
	}
	return metrics
}

func TestCollectAndSend(t *testing.T) {
	tests := []struct {
		name       string
		properties *Properties
		runtime    time.Duration
		want       error
	}{
		{
			name: "TenCollectorsRunForTenSeconds",
			properties: &Properties{
				Client:     &fake.TimeSeriesCreator{},
				Collectors: fakeCollectors(10, 1),
				Config:     quickTestConfig,
			},
			runtime: 10 * time.Second,
		},
		{
			name: "ZeroCollectors",
			properties: &Properties{
				Client:     &fake.TimeSeriesCreator{},
				Collectors: nil,
				Config:     quickTestConfig,
			},
			runtime: 2 * time.Second,
			want:    cmpopts.AnyError,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), test.runtime)
			defer cancel()

			got := test.properties.collectAndSend(ctx)

			if !cmp.Equal(got, test.want, cmpopts.EquateErrors()) {
				t.Errorf("Failure in collectAndSend(), got: %v want: %v.", got, test.want)
			}
		})
	}
}

func TestCollectAndSendOnce(t *testing.T) {
	tests := []struct {
		name           string
		properties     *Properties
		wantSent       int
		wantBatchCount int
		wantErr        error
	}{
		{
			name: "TenCollectorsSuccess",
			properties: &Properties{
				Client:     &fake.TimeSeriesCreator{},
				Collectors: fakeCollectors(10, 1),
				Config:     quickTestConfig,
			},
			wantSent:       10,
			wantBatchCount: 1,
		},
		{
			name: "SendFailure",
			properties: &Properties{
				Client:     &fake.TimeSeriesCreator{Err: cmpopts.AnyError},
				Collectors: fakeCollectors(1, 1),
				Config:     quickTestConfig,
			},
			wantErr:        cmpopts.AnyError,
			wantBatchCount: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotSent, gotBatchCount, gotErr := test.properties.collectAndSendOnce(context.Background())

			if !cmp.Equal(gotErr, test.wantErr, cmpopts.EquateErrors()) {
				t.Errorf("Failure in collectAndSendOnce(), gotErr: %v wantErr: %v.", gotErr, test.wantErr)
			}

			if gotBatchCount != test.wantBatchCount {
				t.Errorf("Failure in collectAndSendOnce(), gotBatchCount: %v wantBatchCount: %v.",
					gotBatchCount, test.wantBatchCount)
			}

			if gotSent != test.wantSent {
				t.Errorf("Failure in collectAndSendOnce(), gotSent: %v wantSent: %v.", gotSent, test.wantSent)
			}
		})
	}
}

func TestSend(t *testing.T) {
	tests := []struct {
		name           string
		count          int
		client         *fake.TimeSeriesCreator
		want           int
		wantBatchCount int
		wantErr        error
	}{
		{
			name:           "SingleBatch",
			count:          199,
			client:         &fake.TimeSeriesCreator{},
			want:           199,
			wantBatchCount: 1,
		},
		{
			name:           "MultipleBatches",
			count:          399,
			client:         &fake.TimeSeriesCreator{},
			want:           399,
			wantBatchCount: 2,
		},
		{
			name:           "SendErrorSingleBatch",
			count:          5,
			client:         &fake.TimeSeriesCreator{Err: cmpopts.AnyError},
			wantErr:        cmpopts.AnyError,
			wantBatchCount: 1,
		},
		{
			name:           "SendErrorMultipleBatches",
			count:          399,
			client:         &fake.TimeSeriesCreator{Err: cmpopts.AnyError},
			wantErr:        cmpopts.AnyError,
			wantBatchCount: 1,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			p := &Properties{
				Client: test.client,
				Config: quickTestConfig,
			}

			metrics := createFakeMetrics(test.count)
			got, gotBatchCount, gotErr := p.send(context.Background(), metrics)

			if !cmp.Equal(gotErr, test.wantErr, cmpopts.EquateErrors()) {
				t.Errorf("Failure in send(), gotErr: %v wantErr: %v.", gotErr, test.wantErr)
			}

			if got != test.want {
				t.Errorf("Failure in send(), got: %v want: %v.", got, test.want)
			}

			if gotBatchCount != test.wantBatchCount {
				t.Errorf("Failure in send(), gotBatchCount: %v wantBatchCount: %v.", gotBatchCount, test.wantBatchCount)
			}
		})
	}
}

func TestInstancesWithCredentials(t *testing.T) {
	tests := []struct {
		name   string
		params *Parameters
		want   *sapb.SAPInstances
	}{
		{
			name: "CredentialsSet",
			params: &Parameters{
				SAPInstances: fakeSAPInstances("HANA"),
				Config: &cpb.Configuration{
					CollectionConfiguration: &cpb.CollectionConfiguration{
						HanaMetricsConfig: &cpb.HANAMetricsConfig{
							HanaDbUser:     "test-db-user",
							HanaDbPassword: "test-pass",
						},
					},
				},
			},
			want: &sapb.SAPInstances{
				Instances: []*sapb.SAPInstance{
					&sapb.SAPInstance{
						Type:           sapb.InstanceType_HANA,
						HanaDbUser:     "test-db-user",
						HanaDbPassword: "test-pass",
					},
				},
			},
		},
		{
			name: "CredentialsNotSet",
			params: &Parameters{
				SAPInstances: fakeSAPInstances("HANA"),
				Config:       quickTestConfig,
			},
			want: fakeSAPInstances("HANA"),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			got := instancesWithCredentials(context.Background(), test.params)

			if diff := cmp.Diff(test.want, got, protocmp.Transform()); diff != "" {
				t.Errorf("instancesWithCredentials() returned diff (-want +got):\n%s", diff)
			}
		})
	}
}
