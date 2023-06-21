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

package sapcontrol

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/GoogleCloudPlatform/sapagent/internal/commandlineexecutor"
	"github.com/GoogleCloudPlatform/sapagent/internal/sapcontrolclient"
)

var (
	defaultProcessListOutput = `OK
		0 name: hdbdaemon
		0 dispstatus: GREEN
		0 pid: 111
		1 name: hdbcompileserver
		1 dispstatus: GREEN
		1 pid: 222
		2 name: hdbindexserver
		2 dispstatus: GREEN
		2 pid: 333`

	defaultEnqTableOutput = `
	OK
lock_name, lock_arg, lock_mode, owner, owner_vb, use_count_owner, use_count_owner_vb, client, user, transaction, object, backup
USR04, 000DDIC, E, 20230424073648639586000402dnwh75ldbci....................., 20230424073648639586000402dnwh75ldbci....................., 0, 1, 000, SAP*, SU01, E_USR04, FALSE
	`
	multilineEnqTableOutput = `
	09.05.2023 15:23:12
	EnqGetLockTable
	OK
	lock_name, lock_arg, lock_mode, owner, owner_vb, use_count_owner, use_count_owner_vb, client, user, transaction, object, backup
	USR04, 001BASIS, E, 20230509120639629460000602dnwh75ldbci....................., 20230509120639629460000602dnwh75ldbci....................., 0, 1, 001, DDIC, SU01, E_USR04, FALSE
	USR04, 001CALM_USER, E, 20230509120620130684000702dnwh75ldbci....................., 20230509120620130684000702dnwh75ldbci....................., 0, 1, 001, DDIC, SU01, E_USR04, FALSE`
)

type fakeRunner struct {
	stdOut, stdErr string
	exitCode       int
	err            error
}

func (f *fakeRunner) RunWithEnv() (string, string, int, error) {
	return f.stdOut, f.stdErr, f.exitCode, f.err
}

func TestProcessList(t *testing.T) {
	tests := []struct {
		name           string
		fakeExec       commandlineexecutor.Execute
		wantProcStatus map[int]*ProcessStatus
		wantExitCode   int
		wantErr        error
	}{
		{
			name: "SucceedsOneProcess",
			fakeExec: func(context.Context, commandlineexecutor.Params) commandlineexecutor.Result {
				return commandlineexecutor.Result{
					StdOut: `0 name: hdbdaemon
					0 description: HDB Daemon
					0 dispstatus: GREEN
					0 pid: 1234`,
					ExitCode: 3,
				}
			},
			wantProcStatus: map[int]*ProcessStatus{
				0: &ProcessStatus{
					Name:          "hdbdaemon",
					DisplayStatus: "GREEN",
					IsGreen:       true,
					PID:           "1234",
				},
			},
			wantExitCode: 3,
			wantErr:      nil,
		},
		{
			name: "SucceedsAllProcesses",
			fakeExec: func(context.Context, commandlineexecutor.Params) commandlineexecutor.Result {
				return commandlineexecutor.Result{
					StdOut:   defaultProcessListOutput,
					ExitCode: 4,
				}
			},
			wantProcStatus: map[int]*ProcessStatus{
				0: &ProcessStatus{
					Name:          "hdbdaemon",
					DisplayStatus: "GREEN",
					IsGreen:       true,
					PID:           "111",
				},
				1: &ProcessStatus{
					Name:          "hdbcompileserver",
					DisplayStatus: "GREEN",
					IsGreen:       true,
					PID:           "222",
				},
				2: &ProcessStatus{
					Name:          "hdbindexserver",
					DisplayStatus: "GREEN",
					IsGreen:       true,
					PID:           "333",
				},
			},
			wantExitCode: 4,
			wantErr:      nil,
		},
		{
			name: "SapControlFails",
			fakeExec: func(context.Context, commandlineexecutor.Params) commandlineexecutor.Result {
				return commandlineexecutor.Result{
					Error: cmpopts.AnyError,
				}
			},
			wantErr: cmpopts.AnyError,
		},
		{
			name: "SapControlInvalidExitCode",
			fakeExec: func(context.Context, commandlineexecutor.Params) commandlineexecutor.Result {
				return commandlineexecutor.Result{
					ExitCode: -1,
				}
			},
			wantErr:      cmpopts.AnyError,
			wantExitCode: -1,
		},
		{
			name: "NameError",
			fakeExec: func(context.Context, commandlineexecutor.Params) commandlineexecutor.Result {
				return commandlineexecutor.Result{
					StdOut: `abc name: msg_server
					0 description: Message Server
					0 dispstatus: GREEN`,
				}
			},
			wantErr: cmpopts.AnyError,
		},
		{
			name: "CountMismatch",
			fakeExec: func(context.Context, commandlineexecutor.Params) commandlineexecutor.Result {
				return commandlineexecutor.Result{
					StdOut: `0 name: msg_server
					0 description: Message Server
					0 dispstatus: GREEN
					1 dispstatus: GREEN`,
				}
			},
			wantErr: cmpopts.AnyError,
		},
		{
			name: "PidMismatch",
			fakeExec: func(context.Context, commandlineexecutor.Params) commandlineexecutor.Result {
				return commandlineexecutor.Result{
					StdOut: `0 name: msg_server
					0 description: Message Server
					1 dispstatus: GREEN`,
				}
			},
			wantErr: cmpopts.AnyError,
		},
		{
			name: "NameIntegerOverflow",
			fakeExec: func(context.Context, commandlineexecutor.Params) commandlineexecutor.Result {
				return commandlineexecutor.Result{
					StdOut: `1000000000000000000000000 name: msg_server
					0 description: Message Server
					0 dispstatus: GREEN,
					0 pid: 1234`,
				}
			},
			wantErr: cmpopts.AnyError,
		},
		{
			name: "DispStatusIntegerOverflow",
			fakeExec: func(context.Context, commandlineexecutor.Params) commandlineexecutor.Result {
				return commandlineexecutor.Result{
					StdOut: `0 name: msg_server
					0 description: Message Server
					1000000000000000000000000 dispstatus: GREEN,
					0 pid: 1234`,
				}
			},
			wantErr: cmpopts.AnyError,
		},
		{
			name: "NoNameEntryForProcess",
			fakeExec: func(context.Context, commandlineexecutor.Params) commandlineexecutor.Result {
				return commandlineexecutor.Result{
					StdOut: `1 name: hdbdaemon
					0 description: HDB Daemon
					0 dispstatus: GREEN
					0 pid: 1234`,
					ExitCode: 3,
				}
			},
			wantErr: cmpopts.AnyError,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := Properties{}
			gotProcStatus, gotExitCode, gotErr := p.ProcessList(context.Background(), test.fakeExec, commandlineexecutor.Params{})

			if !cmp.Equal(gotErr, test.wantErr, cmpopts.EquateErrors()) {
				t.Errorf("ProcessList(), gotErr: %v wantErr: %v.",
					gotErr, test.wantErr)
			}

			if gotExitCode != test.wantExitCode {
				t.Errorf("ProcessList(), gotExitCode: %d wantExitCode: %d.",
					gotExitCode, test.wantExitCode)
			}

			diff := cmp.Diff(test.wantProcStatus, gotProcStatus, cmp.AllowUnexported(ProcessStatus{}))
			if diff != "" {
				t.Errorf("ProcessList(), diff (-want +got):\n%s", diff)
			}
		})
	}
}

type fakeSAPClient struct {
	processes []sapcontrolclient.OSProcess
	err       error
}

func newFakeSAPClient(processes []sapcontrolclient.OSProcess, err error) fakeSAPClient {
	return fakeSAPClient{
		processes: processes,
		err:       err,
	}
}

func (c fakeSAPClient) GetProcessList() ([]sapcontrolclient.OSProcess, error) {
	return c.processes, c.err
}

func TestCreateProcessMapFromAPIResp(t *testing.T) {
	tests := []struct {
		name           string
		respProcesses  []sapcontrolclient.OSProcess
		wantProcStatus map[int]*ProcessStatus
		wantErr        error
	}{
		{
			name: "SucceedsAllProcesses",
			respProcesses: []sapcontrolclient.OSProcess{
				{"hdbdaemon", "SAPControl-GREEN", 9609},
				{"hdbcompileserver", "SAPControl-GREEN", 9972},
				{"hdbindexserver", "SAPControl-GREEN", 10013},
				{"hdbnameserver", "SAPControl-GREEN", 9642},
				{"hdbpreprocessor", "SAPControl-GREEN", 9975},
			},
			wantProcStatus: map[int]*ProcessStatus{
				0: &ProcessStatus{Name: "hdbdaemon", DisplayStatus: "GREEN", IsGreen: true, PID: "9609"},
				1: &ProcessStatus{Name: "hdbcompileserver", DisplayStatus: "GREEN", IsGreen: true, PID: "9972"},
				2: &ProcessStatus{Name: "hdbindexserver", DisplayStatus: "GREEN", IsGreen: true, PID: "10013"},
				3: &ProcessStatus{Name: "hdbnameserver", DisplayStatus: "GREEN", IsGreen: true, PID: "9642"},
				4: &ProcessStatus{Name: "hdbpreprocessor", DisplayStatus: "GREEN", IsGreen: true, PID: "9975"},
			},
			wantErr: nil,
		},
		{
			name: "NoNameForProcess",
			respProcesses: []sapcontrolclient.OSProcess{
				{"", "SAPControl-GREEN", 9609},
				{"hdbcompileserver", "SAPControl-GREEN", 9972},
			},
			wantProcStatus: map[int]*ProcessStatus{
				1: &ProcessStatus{Name: "hdbcompileserver", DisplayStatus: "GREEN", IsGreen: true, PID: "9972"},
			},
			wantErr: nil,
		},
		{
			name: "NoPIDForProcess",
			respProcesses: []sapcontrolclient.OSProcess{
				{"hdbdaemon", "SAPControl-GREEN", 9609},
				{"hdbcompileserver", "SAPControl-GREEN", 0},
			},
			wantProcStatus: map[int]*ProcessStatus{
				0: &ProcessStatus{Name: "hdbdaemon", DisplayStatus: "GREEN", IsGreen: true, PID: "9609"},
			},
			wantErr: nil,
		},
		{
			name: "NoDispstatus",
			respProcesses: []sapcontrolclient.OSProcess{
				{"hdbdaemon", "SAPControl-GREEN", 9609},
				{"hdbcompileserver", "", 9972},
			},
			wantProcStatus: map[int]*ProcessStatus{
				0: &ProcessStatus{Name: "hdbdaemon", DisplayStatus: "GREEN", IsGreen: true, PID: "9609"},
			},
			wantErr: nil,
		},
		{
			name: "WrongFormatDispstatus",
			respProcesses: []sapcontrolclient.OSProcess{
				{"hdbdaemon", "SAP-Control-GREEN", 9609},
				{"hdbcompileserver", "SAPControl-GREEN", 9972},
			},
			wantProcStatus: map[int]*ProcessStatus{
				1: &ProcessStatus{Name: "hdbcompileserver", DisplayStatus: "GREEN", IsGreen: true, PID: "9972"},
			},
			wantErr: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotProcStatus, gotErr := createProcessMapFromAPIResp(test.respProcesses)

			if !cmp.Equal(gotErr, test.wantErr, cmpopts.EquateErrors()) {
				t.Errorf("createProcessMapFromAPIResp(%v), gotErr: %v wantErr: %v.", test.respProcesses, gotErr, test.wantErr)
			}

			if diff := cmp.Diff(test.wantProcStatus, gotProcStatus); diff != "" {
				t.Errorf("createProcessMapFromAPIResp(%v) returned unexpected diff (-want +got):\n%v \ngot : %v", test.respProcesses, diff, gotProcStatus)
			}
		})
	}
}

func TestGetProcessList(t *testing.T) {
	tests := []struct {
		name           string
		fakeSAPClient  ClientInterface
		wantProcStatus map[int]*ProcessStatus
		wantErr        error
	}{
		{
			name:           "FakeCall",
			fakeSAPClient:  newFakeSAPClient([]sapcontrolclient.OSProcess{}, nil),
			wantProcStatus: map[int]*ProcessStatus{},
			wantErr:        nil,
		},
		{
			name:           "Error",
			fakeSAPClient:  newFakeSAPClient(nil, cmpopts.AnyError),
			wantProcStatus: nil,
			wantErr:        cmpopts.AnyError,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := Properties{}
			gotProcStatus, gotErr := p.GetProcessList(test.fakeSAPClient)
			if !cmp.Equal(gotErr, test.wantErr, cmpopts.EquateErrors()) {
				t.Errorf("GetProcessList(%v), gotErr: %v wantErr: %v.", test.fakeSAPClient, gotErr, test.wantErr)
			}
			if diff := cmp.Diff(test.wantProcStatus, gotProcStatus); diff != "" {
				t.Errorf("Properties.GetProcessList(%v) returned unexpected diff (-want +got):\n%v \ngot : %v", test.fakeSAPClient, diff, gotProcStatus)
			}
		})
	}
}

func TestParseABAPGetWPTable(t *testing.T) {
	tests := []struct {
		name              string
		fakeExec          commandlineexecutor.Execute
		wantProcesses     map[string]int
		wantBusyProcesses map[string]int
		wantPIDMap        map[string]string
		wantErr           error
	}{
		{
			name: "Success",
			fakeExec: func(context.Context, commandlineexecutor.Params) commandlineexecutor.Result {
				return commandlineexecutor.Result{
					StdOut: `No, Typ, Pid, Status, Reason, Start, Err, Sem, Cpu, Time, Program, Client, User, Action, Table
					0, DIA, 7488, Wait, , yes, , , 0:24:54,4, , , , ,
					1, BTC, 7489, Wait, , yes, , , 0:33:24, , , , , ,
					2, SPO, 7490, Wait, , yes, , , 0:22:11, , , , , ,
					3, DIA, 7491, Wait, , yes, , , 0:46:38, , , , , ,
					4, DIA, 7492, Wait, , yes, , , 0:37:05, , , , , ,`,
				}
			},
			wantProcesses:     map[string]int{"DIA": 3, "BTC": 1, "SPO": 1},
			wantBusyProcesses: map[string]int{"DIA": 1},
			wantPIDMap:        map[string]string{"7488": "DIA", "7489": "BTC", "7490": "SPO", "7491": "DIA", "7492": "DIA"},
		},
		{
			name: "Error",
			fakeExec: func(context.Context, commandlineexecutor.Params) commandlineexecutor.Result {
				return commandlineexecutor.Result{
					Error: cmpopts.AnyError,
				}
			},
			wantErr: cmpopts.AnyError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := Properties{}
			gotProcessCount, gotBusyProcessCount, gotPIDMap, err := p.ParseABAPGetWPTable(context.Background(), test.fakeExec, commandlineexecutor.Params{})

			if !cmp.Equal(err, test.wantErr, cmpopts.EquateErrors()) {
				t.Errorf("ParseABAPGetWPTable(%v)=%v, want: %v.", test.fakeExec, err, test.wantErr)
			}
			if diff := cmp.Diff(test.wantProcesses, gotProcessCount); diff != "" {
				t.Errorf("ParseABAPGetWPTable(%v)=%v, want: %v.", test.fakeExec, gotProcessCount, test.wantProcesses)
			}
			if diff := cmp.Diff(test.wantBusyProcesses, gotBusyProcessCount); diff != "" {
				t.Errorf("ParseABAPGetWPTable(%v)=%v, want: %v.", test.fakeExec, gotBusyProcessCount, test.wantBusyProcesses)
			}
			if diff := cmp.Diff(test.wantPIDMap, gotPIDMap); diff != "" {
				t.Errorf("ParseABAPGetWPTable(%v)=%v, want: %v.", test.fakeExec, gotPIDMap, test.wantPIDMap)
			}
		})
	}
}

func TestParseQueueStats(t *testing.T) {
	tests := []struct {
		name        string
		fakeExec    commandlineexecutor.Execute
		wantCurrent map[string]int
		wantPeak    map[string]int
		wantErr     error
	}{
		{
			name: "Success",
			fakeExec: func(context.Context, commandlineexecutor.Params) commandlineexecutor.Result {
				return commandlineexecutor.Result{
					StdOut: `Typ, Now, High, Max, Writes, Reads
					ABAP/NOWP, 0, 8, 14000, 270537, 270537
					ABAP/DIA, 0, 10, 14000, 534960, 534960
					ICM/Intern, 0, 7, 6000, 184690, 184690`,
				}
			},
			wantCurrent: map[string]int{"ABAP/NOWP": 0, "ABAP/DIA": 0, "ICM/Intern": 0},
			wantPeak:    map[string]int{"ABAP/NOWP": 8, "ABAP/DIA": 10, "ICM/Intern": 7},
		},
		{
			name: "Error",
			fakeExec: func(context.Context, commandlineexecutor.Params) commandlineexecutor.Result {
				return commandlineexecutor.Result{
					Error: cmpopts.AnyError,
				}
			},
			wantErr: cmpopts.AnyError,
		},
		{
			name: "CurrentCountIntegerOverflow",
			fakeExec: func(context.Context, commandlineexecutor.Params) commandlineexecutor.Result {
				return commandlineexecutor.Result{
					StdOut: `ABAP/NOWP, 1000000000000000000000, 8, 14000, 270537, 270537
					ABAP/DIA, 0, 10, 14000, 534960, 534960`,
				}
			},
			wantCurrent: map[string]int{"ABAP/DIA": 0},
			wantPeak:    map[string]int{"ABAP/DIA": 10},
		},
		{
			name: "PeakCountIntegerOverflow",
			fakeExec: func(context.Context, commandlineexecutor.Params) commandlineexecutor.Result {
				return commandlineexecutor.Result{
					StdOut: `ABAP/NOWP, 0, 1000000000000000000000, 14000, 270537, 270537
					ABAP/DIA, 0, 10, 14000, 534960, 534960`,
				}
			},
			wantCurrent: map[string]int{"ABAP/DIA": 0, "ABAP/NOWP": 0},
			wantPeak:    map[string]int{"ABAP/DIA": 10},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := Properties{}
			gotCurrentQueueUsage, gotPeakQueueUsage, err := p.ParseQueueStats(context.Background(), test.fakeExec, commandlineexecutor.Params{})

			if !cmp.Equal(err, test.wantErr, cmpopts.EquateErrors()) {
				t.Errorf("ParseQueueStats(%v)=%v, want: %v.", test.fakeExec, err, test.wantErr)
			}
			if diff := cmp.Diff(test.wantCurrent, gotCurrentQueueUsage); diff != "" {
				t.Errorf("ParseQueueStats(%v)=%v, want: %v.", test.fakeExec, gotCurrentQueueUsage, test.wantCurrent)
			}
			if diff := cmp.Diff(test.wantPeak, gotPeakQueueUsage); diff != "" {
				t.Errorf("ParseQueueStats(%v)=%v, want: %v.", test.fakeExec, gotPeakQueueUsage, test.wantPeak)
			}
		})
	}
}

func TestEnqGetLockTable(t *testing.T) {
	tests := []struct {
		name         string
		fakeExec     commandlineexecutor.Execute
		wantEnqLocks []*EnqLock
		wantErr      error
	}{
		{
			name: "Success",
			fakeExec: func(context.Context, commandlineexecutor.Params) commandlineexecutor.Result {
				return commandlineexecutor.Result{
					StdOut: defaultEnqTableOutput,
				}
			},
			wantEnqLocks: []*EnqLock{
				{
					LockName:         "USR04",
					LockArg:          "000DDIC",
					LockMode:         "E",
					Owner:            "20230424073648639586000402dnwh75ldbci.....................",
					OwnerVB:          "20230424073648639586000402dnwh75ldbci.....................",
					UserCountOwner:   0,
					UserCountOwnerVB: 1,
					Client:           "000",
					User:             "SAP*",
					Transaction:      "SU01",
					Object:           "E_USR04",
					Backup:           "FALSE",
				},
			},
		},
		{
			name: "MultipleLocksSuccess",
			fakeExec: func(context.Context, commandlineexecutor.Params) commandlineexecutor.Result {
				return commandlineexecutor.Result{
					StdOut: multilineEnqTableOutput,
				}
			},
			wantEnqLocks: []*EnqLock{
				{
					LockName:         "USR04",
					LockArg:          "001BASIS",
					LockMode:         "E",
					Owner:            "20230509120639629460000602dnwh75ldbci.....................",
					OwnerVB:          "20230509120639629460000602dnwh75ldbci.....................",
					UserCountOwnerVB: 1,
					Client:           "001",
					User:             "DDIC",
					Transaction:      "SU01",
					Object:           "E_USR04",
					Backup:           "FALSE",
				},
				{
					LockName:         "USR04",
					LockArg:          "001CALM_USER",
					LockMode:         "E",
					Owner:            "20230509120620130684000702dnwh75ldbci.....................",
					OwnerVB:          "20230509120620130684000702dnwh75ldbci.....................",
					UserCountOwnerVB: 1,
					Client:           "001",
					User:             "DDIC",
					Transaction:      "SU01",
					Object:           "E_USR04",
					Backup:           "FALSE",
				},
			},
		},
		{
			name: "Error",
			fakeExec: func(context.Context, commandlineexecutor.Params) commandlineexecutor.Result {
				return commandlineexecutor.Result{
					Error: cmpopts.AnyError,
				}
			},
			wantErr: cmpopts.AnyError,
		},
		{
			name: "ErroneousOwnerCount",
			fakeExec: func(context.Context, commandlineexecutor.Params) commandlineexecutor.Result {
				return commandlineexecutor.Result{
					StdOut: "USR04, 000DDIC, E, dnwh75ldbci, dnwh75ldbci, 1ab0, 1, 000, SAP*, SU01, E_USR04, FALSE",
				}
			},
			wantErr: cmpopts.AnyError,
		},
		{
			name: "ErroneousOwnerCountVB",
			fakeExec: func(context.Context, commandlineexecutor.Params) commandlineexecutor.Result {
				return commandlineexecutor.Result{
					StdOut: "USR04, 000DDIC, E, dnwh75ldbci, dnwh75ldbci, 10, 1000000000000000000000, 000, SAP*, SU01, E_USR04, FALSE",
				}
			},
			wantErr: cmpopts.AnyError,
		},
		{
			name: "InvalidStatus",
			fakeExec: func(context.Context, commandlineexecutor.Params) commandlineexecutor.Result {
				return commandlineexecutor.Result{
					ExitCode: -1,
				}
			},
			wantErr: cmpopts.AnyError,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := Properties{}
			gotEnqList, gotErr := p.EnqGetLockTable(context.Background(), test.fakeExec, commandlineexecutor.Params{})

			if !cmp.Equal(gotErr, test.wantErr, cmpopts.EquateErrors()) {
				t.Errorf("EnqGetLockTable(%v)=%v, want: %v.", test.fakeExec, gotErr, test.wantErr)
			}
			if diff := cmp.Diff(test.wantEnqLocks, gotEnqList); diff != "" {
				t.Errorf("EnqGetLockTable(%v) mismatch, diff (-want, +got): %v", test.fakeExec, diff)
			}
		})
	}
}
