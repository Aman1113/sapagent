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

package hostdiscovery

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/GoogleCloudPlatform/sapagent/shared/commandlineexecutor"
)

const (
	defaultClusterOutput = `
	line1
	line2
	rsc_vip_int-primary IPaddr2
	anotherline
	params ip 127.0.0.1 other text
	line3
	line4
	`
	defaultFilestoreOutput = `
Filesystem                        Size  Used Avail Use% Mounted on
udev                               48G     0   48G   0% /dev
tmpfs                             9.5G  4.2M  9.5G   1% /run
1.2.3.4:/vol                        8G     0    8G   0% /vol
tmpfs                              48G  2.0M   48G   1% /dev/shm
	`
)

func TestDiscoverClusterCRM(t *testing.T) {
	tests := []struct {
		name        string
		testExecute commandlineexecutor.Execute
		wantAddr    string
		wantErr     error
	}{
		{
			name: "CRM Success",
			testExecute: func(context.Context, commandlineexecutor.Params) commandlineexecutor.Result {
				return commandlineexecutor.Result{
					StdOut: defaultClusterOutput,
					StdErr: "",
				}
			},
			wantAddr: "127.0.0.1",
			wantErr:  nil,
		}, {
			name: "CRM execute error",
			testExecute: func(context.Context, commandlineexecutor.Params) commandlineexecutor.Result {
				return commandlineexecutor.Result{
					StdOut: "",
					StdErr: "error",
					Error:  errors.New("exit status 1"),
				}
			},
			wantAddr: "",
			wantErr:  cmpopts.AnyError,
		}, {
			name: "CRM no params ip",
			testExecute: func(context.Context, commandlineexecutor.Params) commandlineexecutor.Result {
				return commandlineexecutor.Result{
					StdOut: "rsc_cip_int-primary IPAddr2",
					StdErr: "",
				}
			},
			wantAddr: "",
			wantErr:  cmpopts.AnyError,
		}, {
			name: "CRM no output",
			testExecute: func(context.Context, commandlineexecutor.Params) commandlineexecutor.Result {
				return commandlineexecutor.Result{
					StdOut: "line1\nline2\n",
					StdErr: "",
				}
			},
			wantAddr: "",
			wantErr:  cmpopts.AnyError,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d := HostDiscovery{
				Execute: test.testExecute,
			}
			got, err := d.discoverClusterCRM(context.Background())
			if diff := cmp.Diff(got, test.wantAddr); diff != "" {
				t.Errorf("discoverClusterCRM mismatch (-want, +got):\n%s", diff)
			}
			if !cmp.Equal(err, test.wantErr, cmpopts.EquateErrors()) {
				t.Error("discoverClusterCRM error mismatch")
			}
		})
	}
}

func TestDiscoverClusterPCS(t *testing.T) {
	tests := []struct {
		name        string
		testExecute commandlineexecutor.Execute
		wantAddr    string
		wantErr     error
	}{
		{
			name: "PCS Success",
			testExecute: func(context.Context, commandlineexecutor.Params) commandlineexecutor.Result {
				return commandlineexecutor.Result{
					StdOut: defaultClusterOutput,
					StdErr: "",
				}
			},
			wantAddr: "127.0.0.1",
			wantErr:  nil,
		}, {
			name: "PCS execute error",
			testExecute: func(context.Context, commandlineexecutor.Params) commandlineexecutor.Result {
				return commandlineexecutor.Result{
					StdOut: "",
					StdErr: "error",
					Error:  errors.New("exit status 1"),
				}
			},
			wantAddr: "",
			wantErr:  cmpopts.AnyError,
		}, {
			name: "PCS no params ip",
			testExecute: func(context.Context, commandlineexecutor.Params) commandlineexecutor.Result {
				return commandlineexecutor.Result{
					StdOut: "rsc_cip_int-primary IPAddr2",
					StdErr: "",
				}
			},
			wantAddr: "",
			wantErr:  cmpopts.AnyError,
		}, {
			name: "PCS no output",
			testExecute: func(context.Context, commandlineexecutor.Params) commandlineexecutor.Result {
				return commandlineexecutor.Result{
					StdOut: "line1\nline2\n",
					StdErr: "",
				}
			},
			wantAddr: "",
			wantErr:  cmpopts.AnyError,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d := HostDiscovery{
				Execute: test.testExecute,
			}
			got, err := d.discoverClusterPCS(context.Background())
			if diff := cmp.Diff(got, test.wantAddr); diff != "" {
				t.Errorf("discoverClusterPCS mismatch (-want, +got):\n%s", diff)
			}
			if !cmp.Equal(err, test.wantErr, cmpopts.EquateErrors()) {
				t.Error("discoverClusterPCS error mismatch")
			}
		})
	}
}

func TestDiscoverClusterAddress(t *testing.T) {
	tests := []struct {
		name        string
		testExists  commandlineexecutor.Exists
		testExecute commandlineexecutor.Execute
		wantAddr    string
		wantErr     error
	}{{
		name:       "Address from CRM",
		testExists: func(cmd string) bool { return cmd == "crm" },
		testExecute: func(ctx context.Context, params commandlineexecutor.Params) commandlineexecutor.Result {
			if params.Executable != "crm" {
				return commandlineexecutor.Result{
					StdOut: "",
					StdErr: "Unexpected command",
					Error:  errors.New("Unexpected command"),
				}
			}
			return commandlineexecutor.Result{
				StdOut: defaultClusterOutput,
				StdErr: "",
			}
		},
		wantAddr: "127.0.0.1",
		wantErr:  nil,
	}, {
		name:       "Address from PCS",
		testExists: func(cmd string) bool { return cmd == "pcs" },
		testExecute: func(ctx context.Context, params commandlineexecutor.Params) commandlineexecutor.Result {
			if params.Executable != "pcs" {
				return commandlineexecutor.Result{
					StdOut: "",
					StdErr: "Unexpected command",
					Error:  errors.New("Unexpected command"),
				}
			}
			return commandlineexecutor.Result{
				StdOut: defaultClusterOutput,
				StdErr: "",
			}
		},
		wantAddr: "127.0.0.1",
		wantErr:  nil,
	}, {
		name:       "No valid commands",
		testExists: func(string) bool { return false },
		testExecute: func(context.Context, commandlineexecutor.Params) commandlineexecutor.Result {
			return commandlineexecutor.Result{
				StdOut: "",
				StdErr: "Unexpected command",
				Error:  errors.New("Unexpected command"),
			}
		},
		wantAddr: "",
		wantErr:  cmpopts.AnyError,
	}, {
		name:       "CRM Error",
		testExists: func(cmd string) bool { return cmd == "crm" },
		testExecute: func(ctx context.Context, params commandlineexecutor.Params) commandlineexecutor.Result {
			if params.Executable != "crm" {
				return commandlineexecutor.Result{
					StdOut: "",
					StdErr: "Unexpected command",
					Error:  errors.New("Unexpected command"),
				}
			}
			return commandlineexecutor.Result{
				StdOut: "",
				StdErr: "CRM Error",
				Error:  errors.New("CRM Error"),
			}
		},
		wantAddr: "",
		wantErr:  cmpopts.AnyError,
	}, {
		name:       "PCS Error",
		testExists: func(cmd string) bool { return cmd == "pcs" },
		testExecute: func(ctx context.Context, params commandlineexecutor.Params) commandlineexecutor.Result {
			if params.Executable != "pcs" {
				return commandlineexecutor.Result{
					StdOut: "",
					StdErr: "Unexpected command",
					Error:  errors.New("Unexpected command"),
				}
			}
			return commandlineexecutor.Result{
				StdOut: "",
				StdErr: "PCS Error", Error: errors.New("PCS Error"),
			}
		},
		wantAddr: "",
		wantErr:  cmpopts.AnyError,
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d := HostDiscovery{
				Exists:  test.testExists,
				Execute: test.testExecute,
			}
			got, err := d.discoverClusterAddress(context.Background())
			if diff := cmp.Diff(got, test.wantAddr); diff != "" {
				t.Errorf("discoverClusterAddress mismatch (-want, +got):\n%s", diff)
			}
			if !cmp.Equal(err, test.wantErr, cmpopts.EquateErrors()) {
				t.Error("discoverClusterAddress error mismatch")
			}
		})
	}
}

func TestDiscoverFilestores(t *testing.T) {
	tests := []struct {
		name    string
		exists  commandlineexecutor.Exists
		execute commandlineexecutor.Execute
		want    []string
	}{{
		name:   "Success",
		exists: func(cmd string) bool { return true },
		execute: func(context.Context, commandlineexecutor.Params) commandlineexecutor.Result {
			return commandlineexecutor.Result{
				StdOut: defaultFilestoreOutput,
				StdErr: "",
			}
		},
		want: []string{"1.2.3.4"},
	}, {
		name:   "Multiple NFS",
		exists: func(cmd string) bool { return true },
		execute: func(context.Context, commandlineexecutor.Params) commandlineexecutor.Result {
			return commandlineexecutor.Result{
				StdOut: `
Filesystem                        Size  Used Avail Use% Mounted on
1.2.3.4:/vol                        8G     0    8G   0% /vol
5.6.7.8:/vol2                       8G     0    8G   0% /vol2
tmpfs                              48G  2.0M   48G   1% /dev/shm`,
				StdErr: "",
			}
		},
		want: []string{"1.2.3.4", "5.6.7.8"},
	}, {
		name:   "df does not exist",
		exists: func(cmd string) bool { return false },
		execute: func(context.Context, commandlineexecutor.Params) commandlineexecutor.Result {
			return commandlineexecutor.Result{
				StdOut: "",
				StdErr: "Unexpected command",
				Error:  errors.New("Unexpected command"),
			}
		},
		want: nil,
	}, {
		name:   "df error",
		exists: func(cmd string) bool { return true },
		execute: func(context.Context, commandlineexecutor.Params) commandlineexecutor.Result {
			return commandlineexecutor.Result{
				StdOut: "",
				StdErr: "exit status 1",
				Error:  errors.New("exit status 1"),
			}
		},
		want: nil,
	}, {
		name:   "No NFS",
		exists: func(cmd string) bool { return true },
		execute: func(context.Context, commandlineexecutor.Params) commandlineexecutor.Result {
			return commandlineexecutor.Result{
				StdOut: `
Filesystem                        Size  Used Avail Use% Mounted on
udev                               48G     0   48G   0% /dev
tmpfs                             9.5G  4.2M  9.5G   1% /run`,
				StdErr: "",
			}
		},
		want: []string{},
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d := HostDiscovery{
				Exists:  test.exists,
				Execute: test.execute,
			}
			got := d.discoverFilestores(context.Background())
			if diff := cmp.Diff(got, test.want); diff != "" {
				t.Errorf("discoverFilestores mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestDiscoverCurrentHost(t *testing.T) {
	tests := []struct {
		name    string
		execute commandlineexecutor.Execute
		want    []string
	}{{
		name: "Success",
		execute: func(ctx context.Context, params commandlineexecutor.Params) commandlineexecutor.Result {
			switch params.Executable {
			case "crm":
				return commandlineexecutor.Result{
					StdOut: defaultClusterOutput,
					StdErr: "",
				}
			case "df":
				return commandlineexecutor.Result{
					StdOut: defaultFilestoreOutput,
					StdErr: "",
				}
			default:
				return commandlineexecutor.Result{
					StdErr: "Unexpected command", Error: errors.New("Unexpected command"),
				}
			}
		},
		want: []string{"127.0.0.1", "1.2.3.4"},
	}, {
		name: "clusterError",
		execute: func(ctx context.Context, params commandlineexecutor.Params) commandlineexecutor.Result {
			switch params.Executable {
			case "crm":
				return commandlineexecutor.Result{
					StdErr: "Some error",
					Error:  errors.New("Some Error"),
				}
			case "df":
				return commandlineexecutor.Result{
					StdOut: defaultFilestoreOutput,
					StdErr: "",
				}
			default:
				return commandlineexecutor.Result{
					StdErr: "Unexpected command", Error: errors.New("Unexpected command"),
				}
			}
		},
		want: []string{"1.2.3.4"},
	}, {
		name: "filestoreError",
		execute: func(ctx context.Context, params commandlineexecutor.Params) commandlineexecutor.Result {
			switch params.Executable {
			case "crm":
				return commandlineexecutor.Result{
					StdOut: defaultClusterOutput,
					StdErr: "",
				}
			case "df":
				return commandlineexecutor.Result{
					StdErr: "Some error",
					Error:  errors.New("Some Error"),
				}
			default:
				return commandlineexecutor.Result{
					StdErr: "Unexpected command", Error: errors.New("Unexpected command"),
				}
			}
		},
		want: []string{"127.0.0.1"},
	}}

	ctx := context.Background()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := HostDiscovery{
				Exists:  func(cmd string) bool { return true },
				Execute: tc.execute,
			}
			got := d.DiscoverCurrentHost(ctx)
			if diff := cmp.Diff(tc.want, got, cmpopts.SortSlices(func(a, b string) bool { return a > b })); diff != "" {
				t.Errorf("discoverCurrentHost() returned an unexpected diff (-want +got): %v", diff)
			}
		})
	}
}
