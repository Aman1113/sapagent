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

package onetime

import (
	"context"
	"errors"
	"io/fs"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
	"github.com/google/subcommands"
	"github.com/GoogleCloudPlatform/sapagent/internal/collectiondefinition"
	"github.com/GoogleCloudPlatform/sapagent/internal/configuration"
	"github.com/GoogleCloudPlatform/sapagent/internal/instanceinfo"

	cpb "github.com/GoogleCloudPlatform/sapagent/protos/configuration"
	iipb "github.com/GoogleCloudPlatform/sapagent/protos/instanceinfo"
)

func TestRemoteValidationHandler(t *testing.T) {
	defaultLoadOptions := collectiondefinition.LoadOptions{
		ReadFile: func(s string) ([]byte, error) { return nil, fs.ErrNotExist },
		OSType:   "linux",
		Version:  "1.0",
	}

	tests := []struct {
		name        string
		remote      *RemoteValidation
		loadOptions collectiondefinition.LoadOptions
		want        subcommands.ExitStatus
	}{
		{
			name: "EmptyProject",
			remote: &RemoteValidation{
				instanceid: "instance-1",
				zone:       "zone-1",
			},
			loadOptions: defaultLoadOptions,
			want:        subcommands.ExitUsageError,
		},
		{
			name:        "EmptyInstanceID",
			remote:      &RemoteValidation{},
			loadOptions: defaultLoadOptions,
			want:        subcommands.ExitUsageError,
		},
		{
			name: "EmptyZone",
			remote: &RemoteValidation{
				project: "project-1",
			},
			loadOptions: defaultLoadOptions,
			want:        subcommands.ExitUsageError,
		},
		{
			name: "CollectionDefinitionLoadError",
			remote: &RemoteValidation{
				project:    "project-1",
				instanceid: "instance-1",
				zone:       "zone-1",
			},
			loadOptions: collectiondefinition.LoadOptions{
				ReadFile: func(s string) ([]byte, error) { return nil, errors.New("ReadFile Error") },
				OSType:   "linux",
				Version:  "1.0",
			},
			want: subcommands.ExitFailure,
		},
		{
			name: "Success",
			remote: &RemoteValidation{
				project:    "project-1",
				instanceid: "instance-1",
				zone:       "zone-1",
			},
			loadOptions: defaultLoadOptions,
			want:        subcommands.ExitSuccess,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.remote.remoteValidationHandler(context.Background(), instanceinfo.New(nil, nil), test.loadOptions)
			if got != test.want {
				t.Errorf("remoteValidationHandler(%v) = %v, want %v", test.remote, got, test.want)
			}
		})
	}
}

func TestCreateConfiguration(t *testing.T) {
	project := "test-project"
	instanceID := "test-instanceid"
	instanceName := "test-instancename"
	zone := "test-zone"

	want := &cpb.Configuration{
		CloudProperties: &iipb.CloudProperties{
			ProjectId:    project,
			InstanceId:   instanceID,
			InstanceName: instanceName,
			Zone:         zone,
		},
		AgentProperties: &cpb.AgentProperties{
			Name:    configuration.AgentName,
			Version: configuration.AgentVersion,
		},
	}

	r := &RemoteValidation{project: project, instanceid: instanceID, instancename: instanceName, zone: zone}
	got := r.createConfiguration()
	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("createConfiguration() returned unexpected diff (-want +got):\n%s", diff)
	}
}
