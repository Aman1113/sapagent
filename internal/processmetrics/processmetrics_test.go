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
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"
	"github.com/gammazero/workerpool"
	"github.com/GoogleCloudPlatform/sapagent/internal/cloudmonitoring"
	"github.com/GoogleCloudPlatform/sapagent/internal/cloudmonitoring/fake"
	"github.com/GoogleCloudPlatform/sapagent/internal/heartbeat"

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
			CollectProcessMetrics:       true,
			ProcessMetricsFrequency:     5,
			SlowProcessMetricsFrequency: 30,
			ReliabilityMetricsFrequency: 1,
		},
		CloudProperties: defaultCloudProperties,
	}

	quickTestConfig = &cpb.Configuration{
		CollectionConfiguration: &cpb.CollectionConfiguration{
			CollectProcessMetrics:       true,
			ProcessMetricsFrequency:     1, // Use small value for quick unit tests.
			SlowProcessMetricsFrequency: 6,
			ReliabilityMetricsFrequency: 1,
		},
		CloudProperties: defaultCloudProperties,
	}

	invalidSlowFrequencyTestConfig = &cpb.Configuration{
		CollectionConfiguration: &cpb.CollectionConfiguration{
			CollectProcessMetrics:       true,
			ProcessMetricsFrequency:     5,
			SlowProcessMetricsFrequency: 1,
			ReliabilityMetricsFrequency: 1,
		},
		CloudProperties: defaultCloudProperties,
	}

	defaultBackOffIntervals = cloudmonitoring.NewBackOffIntervals(time.Millisecond, time.Millisecond)
)

type (
	fakeProperties struct {
		SAPInstances *sapb.SAPInstances
		Config       *cpb.Configuration
		Client       cloudmonitoring.TimeSeriesCreator
	}

	fakeCollector struct {
		timeSeriesCount             int
		timesCollectWithRetryCalled int
		m                           *sync.Mutex
	}

	fakeCollectorError struct {
	}

	fakeCollectorErrorWithTimeSeries struct {
		timeSeriesCount int
	}
)

func (f *fakeCollector) Collect(ctx context.Context) ([]*mrpb.TimeSeries, error) {
	m := make([]*mrpb.TimeSeries, f.timeSeriesCount)
	for i := 0; i < f.timeSeriesCount; i++ {
		m[i] = &mrpb.TimeSeries{}
	}
	return m, nil
}

func (f *fakeCollector) CollectWithRetry(ctx context.Context) ([]*mrpb.TimeSeries, error) {
	if f.m != nil {
		f.m.Lock()
		f.timesCollectWithRetryCalled++
		f.m.Unlock()
	} else {
		f.timesCollectWithRetryCalled++
	}
	m := make([]*mrpb.TimeSeries, f.timeSeriesCount)
	for i := 0; i < f.timeSeriesCount; i++ {
		m[i] = &mrpb.TimeSeries{}
	}
	return m, nil
}

func (f *fakeCollectorError) CollectWithRetry(ctx context.Context) ([]*mrpb.TimeSeries, error) {
	return nil, cmpopts.AnyError
}

func (f *fakeCollectorError) Collect(ctx context.Context) ([]*mrpb.TimeSeries, error) {
	return nil, cmpopts.AnyError
}

func (f *fakeCollectorErrorWithTimeSeries) CollectWithRetry(ctx context.Context) ([]*mrpb.TimeSeries, error) {
	m := make([]*mrpb.TimeSeries, f.timeSeriesCount)
	for i := 0; i < f.timeSeriesCount; i++ {
		m[i] = &mrpb.TimeSeries{}
	}
	return m, cmpopts.AnyError
}

func (f *fakeCollectorErrorWithTimeSeries) Collect(ctx context.Context) ([]*mrpb.TimeSeries, error) {
	m := make([]*mrpb.TimeSeries, f.timeSeriesCount)
	for i := 0; i < f.timeSeriesCount; i++ {
		m[i] = &mrpb.TimeSeries{}
	}
	return m, cmpopts.AnyError
}

func fakeCollectors(count, timeSerisCountPerCollector int) []Collector {
	collectors := make([]Collector, count)
	for i := 0; i < count; i++ {
		collectors[i] = &fakeCollector{timeSeriesCount: timeSerisCountPerCollector}
	}
	return collectors
}

func fakeNewMetricClient(ctx context.Context) (cloudmonitoring.TimeSeriesCreator, error) {
	return &fake.TimeSeriesCreatorThreadSafe{}, nil
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
					Type:   sapb.InstanceType_HANA,
					Sapsid: "DEH",
				},
			},
		}
	case "HANACluster":
		return &sapb.SAPInstances{
			Instances: []*sapb.SAPInstance{
				&sapb.SAPInstance{
					Type:   sapb.InstanceType_HANA,
					Sapsid: "DVA",
				},
			},
			LinuxClusterMember: true,
		}
	case "NetweaverCluster":
		return &sapb.SAPInstances{
			Instances: []*sapb.SAPInstance{
				&sapb.SAPInstance{
					Type:   sapb.InstanceType_NETWEAVER,
					Sapsid: "AEK",
				},
			},
			LinuxClusterMember: true,
		}
	case "TwoNetweaverInstancesOnSameMachine":
		return &sapb.SAPInstances{
			Instances: []*sapb.SAPInstance{
				&sapb.SAPInstance{
					Type:   sapb.InstanceType_NETWEAVER,
					Sapsid: "AEK",
				}, &sapb.SAPInstance{
					Type:   sapb.InstanceType_NETWEAVER,
					Sapsid: "AEK",
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
func TestStartProcessMetrics(t *testing.T) {
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
				BackOffs:     defaultBackOffIntervals,
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
				BackOffs:     defaultBackOffIntervals,
			},
			want: false,
		},
		{
			name: "FailsForWindowsOS",
			parameters: Parameters{
				Config:   defaultConfig,
				OSType:   "windows",
				BackOffs: defaultBackOffIntervals,
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
				BackOffs:     defaultBackOffIntervals,
			},
			want: false,
		},
		{
			name: "InvalidProcessMetricFrequencyForSlowMetrics",
			parameters: Parameters{
				Config:       invalidSlowFrequencyTestConfig,
				OSType:       "linux",
				MetricClient: fakeNewMetricClient,
				SAPInstances: fakeSAPInstances("HANA"),
				BackOffs:     defaultBackOffIntervals,
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
				BackOffs:     defaultBackOffIntervals,
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
				BackOffs:     defaultBackOffIntervals,
			},
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			got := startProcessMetrics(ctx, test.parameters)
			if got != test.want {
				t.Errorf("StartProcessMetrics(%v), got: %t want: %t", test.parameters, got, test.want)
			}
		})
	}
}

func TestCreateProcessCollectors(t *testing.T) {
	tests := []struct {
		name                   string
		sapInstances           *sapb.SAPInstances
		wantCollectorCount     int
		wantFastCollectorCount int
	}{
		{
			name:                   "HANAStandaloneInstance",
			sapInstances:           fakeSAPInstances("HANA"),
			wantCollectorCount:     8,
			wantFastCollectorCount: 1,
		},
		{
			name:                   "HANAClusterInstance",
			sapInstances:           fakeSAPInstances("HANACluster"),
			wantCollectorCount:     9,
			wantFastCollectorCount: 1,
		},
		{
			name:                   "NetweaverClusterInstance",
			sapInstances:           fakeSAPInstances("NetweaverCluster"),
			wantCollectorCount:     9,
			wantFastCollectorCount: 1,
		},
		{
			name:                   "TwoNetweaverInstancesOnSameMachine",
			sapInstances:           fakeSAPInstances("TwoNetweaverInstancesOnSameMachine"),
			wantCollectorCount:     11,
			wantFastCollectorCount: 2,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			params := Parameters{
				Config: defaultConfig,
			}
			got := createProcessCollectors(context.Background(), params, &fake.TimeSeriesCreatorThreadSafe{}, test.sapInstances)

			if len(got.Collectors) != test.wantCollectorCount {
				t.Errorf("createProcessCollectors() returned %d collectors, want %d", len(got.Collectors), test.wantCollectorCount)
			}
			if len(got.FastMovingCollectors) != test.wantFastCollectorCount {
				t.Errorf("createProcessCollectors() returned %d fast collectors, want %d", len(got.FastMovingCollectors), test.wantFastCollectorCount)
			}
		})
	}
}

func createFakeMetrics(count int) []*mrpb.TimeSeries {
	var metrics []*mrpb.TimeSeries

	for i := 0; i < count; i++ {
		metrics = append(metrics, &mrpb.TimeSeries{})
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
				Client:     &fake.TimeSeriesCreatorThreadSafe{},
				Collectors: fakeCollectors(10, 1),
				Config:     quickTestConfig,
			},
			runtime: 10 * time.Second,
		},
		{
			name: "ZeroCollectors",
			properties: &Properties{
				Client:     &fake.TimeSeriesCreatorThreadSafe{},
				Collectors: nil,
				Config:     quickTestConfig,
			},
			runtime: 2 * time.Second,
			want:    cmpopts.AnyError,
		},
		{
			name: "SlowCollectorsForThirtySeconds",
			properties: &Properties{
				Client:     &fake.TimeSeriesCreatorThreadSafe{},
				Collectors: fakeCollectors(9, 1),
				Config:     quickTestConfig,
			},
			runtime: 30 * time.Second,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), test.runtime)
			defer cancel()

			got := test.properties.collectAndSendFastMovingMetrics(ctx, defaultBackOffIntervals)

			if !cmp.Equal(got, test.want, cmpopts.EquateErrors()) {
				t.Errorf("Failure in collectAndSendFastMovingMetrics(), got: %v want: %v.", got, test.want)
			}
		})
	}
}

func TestCollectAndSendSlowMovingMetricsOnce(t *testing.T) {
	tests := []struct {
		name           string
		properties     *Properties
		collector      Collector
		wantSent       int
		wantBatchCount int
		wantErr        error
	}{
		{
			name: "CollectorSuccess",
			properties: &Properties{
				Client:     &fake.TimeSeriesCreatorThreadSafe{},
				Collectors: fakeCollectors(10, 1),
				Config:     quickTestConfig,
			},
			collector: &fakeCollector{
				timeSeriesCount: 10,
			},
			wantSent:       10,
			wantBatchCount: 1,
		},
		{
			name: "CollectorFailure",
			properties: &Properties{
				Client:     &fake.TimeSeriesCreator{Err: cmpopts.AnyError},
				Collectors: fakeCollectors(1, 1),
				Config:     quickTestConfig,
			},
			collector:      &fakeCollectorError{},
			wantErr:        cmpopts.AnyError,
			wantBatchCount: 0,
		},
		{
			name: "CollectorFailureWithSomeTimeSeriesData",
			properties: &Properties{
				Client:     &fake.TimeSeriesCreatorThreadSafe{},
				Collectors: fakeCollectors(10, 1),
				Config:     quickTestConfig,
			},
			collector: &fakeCollectorErrorWithTimeSeries{
				timeSeriesCount: 10,
			},
			wantErr:        nil,
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
			collector: &fakeCollector{
				timeSeriesCount: 10,
			},
			wantErr:        cmpopts.AnyError,
			wantBatchCount: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotSent, gotBatchCount, gotErr := collectAndSendSlowMovingMetricsOnce(context.Background(), test.properties, test.collector, defaultBackOffIntervals)

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

func TestCollectAndSendOnceFastMovingMetrics(t *testing.T) {
	tests := []struct {
		name           string
		properties     *Properties
		wantSent       int
		wantBatchCount int
		wantErr        error
	}{
		{
			name: "ThreeCollectorsSuccess",
			properties: &Properties{
				Client:               &fake.TimeSeriesCreatorThreadSafe{},
				FastMovingCollectors: fakeCollectors(3, 1),
				Config:               quickTestConfig,
			},
			wantSent:       3,
			wantBatchCount: 1,
		},
		{
			name: "SendFailure",
			properties: &Properties{
				Client:               &fake.TimeSeriesCreator{Err: cmpopts.AnyError},
				FastMovingCollectors: fakeCollectors(1, 1),
				Config:               quickTestConfig,
			},
			wantErr:        cmpopts.AnyError,
			wantBatchCount: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotSent, gotBatchCount, gotErr := test.properties.collectAndSendFastMovingMetricsOnce(context.Background(), defaultBackOffIntervals)

			if !cmp.Equal(gotErr, test.wantErr, cmpopts.EquateErrors()) {
				t.Errorf("Failure in collectAndSendOnceFastMovingMetrics(), gotErr: %v wantErr: %v.", gotErr, test.wantErr)
			}

			if gotBatchCount != test.wantBatchCount {
				t.Errorf("Failure in collectAndSendOnceFastMovingMetrics(), gotBatchCount: %v wantBatchCount: %v.",
					gotBatchCount, test.wantBatchCount)
			}

			if gotSent != test.wantSent {
				t.Errorf("Failure in collectAndSendOnceFastMovingMetrics(), gotSent: %v wantSent: %v.", gotSent, test.wantSent)
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
				BackOffs: defaultBackOffIntervals,
			},
			want: &sapb.SAPInstances{
				Instances: []*sapb.SAPInstance{
					&sapb.SAPInstance{
						Type:           sapb.InstanceType_HANA,
						HanaDbUser:     "test-db-user",
						HanaDbPassword: "test-pass",
						Sapsid:         "DEH",
					},
				},
			},
		},
		{
			name: "CredentialsNotSet",
			params: &Parameters{
				SAPInstances: fakeSAPInstances("HANA"),
				Config:       quickTestConfig,
				BackOffs:     defaultBackOffIntervals,
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

func TestCollectAndSend_shouldBeatAccordingToHeartbeatSpec(t *testing.T) {
	testData := []struct {
		name         string
		beatInterval time.Duration
		timeout      time.Duration
		want         int
	}{
		{
			name:         "cancel before beat",
			beatInterval: time.Millisecond * 200,
			timeout:      time.Millisecond * 100,
			want:         0,
		},
		{
			name:         "1 beat timeout",
			beatInterval: time.Millisecond * 75,
			timeout:      time.Millisecond * 100,
			want:         1,
		},
		{
			name:         "2 beat timeout",
			beatInterval: time.Millisecond * 45,
			timeout:      time.Millisecond * 110,
			want:         2,
		},
	}
	for _, test := range testData {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			ctx, cancel := context.WithTimeout(ctx, test.timeout)
			defer cancel()
			got := 0
			lock := sync.Mutex{}
			parameters := Parameters{
				Config:       defaultConfig,
				OSType:       "linux",
				MetricClient: fakeNewMetricClient,
				SAPInstances: fakeSAPInstances("HANA"),
				BackOffs:     defaultBackOffIntervals,
				HeartbeatSpec: &heartbeat.Spec{
					BeatFunc: func() {
						lock.Lock()
						defer lock.Unlock()
						got++
					},
					Interval: test.beatInterval,
				},
			}
			properties := createProcessCollectors(context.Background(), parameters, &fake.TimeSeriesCreatorThreadSafe{}, fakeSAPInstances("HANA"))
			properties.collectAndSendFastMovingMetrics(ctx, defaultBackOffIntervals)
			<-ctx.Done()
			lock.Lock()
			defer lock.Unlock()
			if got != test.want {
				t.Errorf("collectAndSendFastMovingMetrics() heartbeat mismatch got %d, want %d", got, test.want)
			}
		})
	}
}

func TestCollectAndSendSlowMovingMetrics(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	m := sync.Mutex{}
	c := &fakeCollector{
		timeSeriesCount: 10,
		m:               &m,
	}
	p := &Properties{
		Client:     &fake.TimeSeriesCreatorThreadSafe{},
		Collectors: []Collector{c},
		Config:     quickTestConfig,
	}
	wp := workerpool.New(1)
	wp.Submit(func() {
		collectAndSendSlowMovingMetrics(ctx, p, c, defaultBackOffIntervals, wp)
	})

	// Wait for some iterations
	time.Sleep(time.Duration(p.Config.GetCollectionConfiguration().GetSlowProcessMetricsFrequency()) * time.Second * 2)
	m.Lock()
	before := c.timesCollectWithRetryCalled
	m.Unlock()
	cancel()

	// Wait some more to ensure workers have stopped
	time.Sleep(time.Duration(p.Config.GetCollectionConfiguration().GetSlowProcessMetricsFrequency()) * time.Second * 2)
	m.Lock()
	after := c.timesCollectWithRetryCalled
	m.Unlock()

	if before != after {
		t.Errorf("collectAndSendSlowMovingMetrics() timesCalled mismatch got %d, want %d", after, before)
	}
}
