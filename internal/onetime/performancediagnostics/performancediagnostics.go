/*
Copyright 2024 Google LLC

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

// Package performancediagnostics performs a series of operations
// and bundles their results in a report which provides
// an insightful assessment of the performance of the system.
package performancediagnostics

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"flag"
	"google.golang.org/protobuf/encoding/protojson"
	"github.com/google/subcommands"
	"github.com/GoogleCloudPlatform/sapagent/internal/backint/configuration"
	"github.com/GoogleCloudPlatform/sapagent/internal/onetime/backint"
	"github.com/GoogleCloudPlatform/sapagent/internal/onetime/configureinstance"
	"github.com/GoogleCloudPlatform/sapagent/internal/onetime"
	"github.com/GoogleCloudPlatform/sapagent/internal/storage"
	"github.com/GoogleCloudPlatform/sapagent/internal/utils/filesystem"
	"github.com/GoogleCloudPlatform/sapagent/internal/utils/zipper"
	"github.com/GoogleCloudPlatform/sapagent/shared/commandlineexecutor"
	"github.com/GoogleCloudPlatform/sapagent/shared/log"

	s "cloud.google.com/go/storage"
	bpb "github.com/GoogleCloudPlatform/sapagent/protos/backint"
	ipb "github.com/GoogleCloudPlatform/sapagent/protos/instanceinfo"
)

const (
	bundlePath = `/tmp/google-cloud-sap-agent/`
)

// Diagnose has args for performance diagnostics OTE subcommands.
type Diagnose struct {
	logLevel, path                      string
	overrideHyperThreading              bool
	paramFile, testBucket, resultBucket string
	scope, bundleName                   string
	help, version                       bool
}

// ReadConfigFile abstracts os.ReadFile function for testability.
type ReadConfigFile func(string) ([]byte, error)

// moveFiles is a struct to store the old and new path of a file.
type moveFiles struct {
	oldPath, newPath string
}

// connectToBucket abstracts storage.ConnectToBucket function for testability.
type connectToBucket func(ctx context.Context, p *storage.ConnectParameters) (attributes, bool)

// attributes interface provides abstraction for ease of testing.
type attributes interface {
	Attrs(ctx context.Context) (attrs *s.BucketAttrs, err error)
}

// options is a struct to store the parameters required for the OTE subcommand.
type options struct {
	read   ReadConfigFile
	fs     filesystem.FileSystem
	exec   commandlineexecutor.Execute
	z      zipper.Zipper
	lp     log.Parameters
	cp     *ipb.CloudProperties
	config *bpb.BackintConfiguration
}

type zipperHelper struct{}

// NewWriter is testable version of zip.NewWriter method.
func (h zipperHelper) NewWriter(w io.Writer) *zip.Writer {
	return zip.NewWriter(w)
}

// FileInfoHeader is testable version of zip.FileInfoHeader method.
func (h zipperHelper) FileInfoHeader(f fs.FileInfo) (*zip.FileHeader, error) {
	return zip.FileInfoHeader(f)
}

// CreateHeader is testable version of CreateHeader method.
func (h zipperHelper) CreateHeader(w *zip.Writer, zfh *zip.FileHeader) (io.Writer, error) {
	return w.CreateHeader(zfh)
}

func (h zipperHelper) Close(w *zip.Writer) error {
	return w.Close()
}

// Name implements the subcommand interface for features.
func (*Diagnose) Name() string {
	return "performancediagnostics"
}

// Synopsis implements the subcommand interface for features.
func (*Diagnose) Synopsis() string {
	return `run a series of diagnostic operations and bundle them in a zip for support team`
}

// Usage implements the subcommand interface for features.
func (*Diagnose) Usage() string {
	return `Usage: performancediagnostics [-scope=<all|backup|io>] [-test-bucket=<name of bucket used to run backup]
	[-param-file=<path to backint parameters file>]	[-result-bucket=<name of bucket to upload the report] [-bundle_name=<diagnostics bundle name>]
	[-path=<path to save bundle>] [-h] [-v] [-loglevel=<debug|info|warn|error]>]` + "\n"
}

// SetFlags implements the subcommand interface for features.
func (d *Diagnose) SetFlags(fs *flag.FlagSet) {
	fs.StringVar(&d.scope, "scope", "all", "Sets the scope for the diagnostic operations. It must be a comma seperated string of values. (optional) Values: <backup|io|all>. Default: all")
	fs.StringVar(&d.testBucket, "test-bucket", "", "Sets the bucket name used to run backup operation. (optional)")
	fs.StringVar(&d.paramFile, "param-file", "", "Sets the path to backint parameters file. Must be present if scope includes backup operation. (optional)")
	fs.StringVar(&d.resultBucket, "result-bucket", "", "Sets the bucket name to upload the final zipped report to. (optional)")
	fs.StringVar(&d.bundleName, "bundle-name", "", "Sets the name for generated bundle. (optional) Default: performance-diagnostics-<current_timestamp>")
	fs.StringVar(&d.path, "path", "", "Sets the path to save the bundle. (optional) Default: /tmp/google-cloud-sap-agent/")
	fs.BoolVar(&d.overrideHyperThreading, "override-hyper-threading", false, "If true, removes 'nosmt' from the 'GRUB_CMDLINE_LINUX_DEFAULT' in '/etc/default/grub' (optional flag, should be true if processing system is OLAP)")
	fs.StringVar(&d.logLevel, "loglevel", "", "Sets the logging level for the agent configuration file. (optional) Default: info")
	fs.BoolVar(&d.help, "help", false, "Display help.")
	fs.BoolVar(&d.help, "h", false, "Display help.")
	fs.BoolVar(&d.version, "version", false, "Print the agent version.")
	fs.BoolVar(&d.version, "v", false, "Print the agent version.")
}

// Execute implements the subcommand interface for feature.
func (d *Diagnose) Execute(ctx context.Context, fs *flag.FlagSet, args ...any) subcommands.ExitStatus {
	lp, cp, exitStatus, completed := onetime.Init(
		ctx,
		onetime.Options{
			Name:     d.Name(),
			Help:     d.help,
			Version:  d.version,
			LogLevel: d.logLevel,
			Fs:       fs,
		},
		args...,
	)
	if !completed {
		return exitStatus
	}
	opts := &options{
		read: os.ReadFile,
		fs:   filesystem.Helper{},
		exec: commandlineexecutor.ExecuteCommand,
		z:    zipperHelper{},
		lp:   lp,
		cp:   cp,
	}
	return d.diagnosticsHandler(ctx, fs, opts)
}

// diagnosticsHandler is the main handler for the performance diagnostics OTE subcommand.
func (d *Diagnose) diagnosticsHandler(ctx context.Context, flagSet *flag.FlagSet, opts *options) subcommands.ExitStatus {
	if !d.validateParams(ctx, flagSet) {
		return subcommands.ExitUsageError
	}
	config, err := d.unmarshalBackintConfig(ctx, os.ReadFile)
	if err != nil {
		onetime.LogErrorToFileAndConsole("error while unmarshalling backint config, failed with error %v", err)
		return subcommands.ExitFailure
	}
	opts.config = config

	destFilesPath := path.Join(d.path, d.bundleName)
	if err := opts.fs.MkdirAll(destFilesPath, 0777); err != nil {
		onetime.LogErrorToFileAndConsole("error while making directory: "+destFilesPath, err)
		return subcommands.ExitFailure
	}
	onetime.LogMessageToFileAndConsole("Collecting Performance Diagnostics Report for Agent for SAP...")
	errsList := performDiagnosticsOps(ctx, d, flagSet, opts)

	oteLog := moveFiles{
		oldPath: fmt.Sprintf("/var/log/google-cloud-sap-agent/%s.log", d.Name()),
		newPath: path.Join(d.path, d.bundleName, fmt.Sprintf("%s.log", d.Name())),
	}
	if err := addToBundle(ctx, []moveFiles{oteLog}, opts.fs); err != nil {
		errsList = append(errsList, fmt.Errorf("failure in adding performance diagnostics OTE to bundle, failed with error %v", err))
	}

	if len(errsList) > 0 {
		onetime.LogMessageToFileAndConsole("Performance Diagnostics Report collection ran into following errors\n")
		for _, err := range errsList {
			onetime.LogErrorToFileAndConsole("Error: ", err)
		}
		return subcommands.ExitFailure
	}
	return subcommands.ExitSuccess
}

// performDiagnosticsOps performs the operations requested by the user.
func performDiagnosticsOps(ctx context.Context, d *Diagnose, flagSet *flag.FlagSet, opts *options) []error {
	errsList := []error{}
	// ConfigureInstance OTE subcommand needs to be run in every case.
	exitStatus := d.runConfigureInstanceOTE(ctx, flagSet, opts.exec, opts.lp, opts.cp)
	if exitStatus != subcommands.ExitSuccess {
		errsList = append(errsList, fmt.Errorf("failure in executing ConfigureInstance OTE, failed with exist status %d", exitStatus))
	}
	// Performance diagnostics operations.
	ops := listOperations(ctx, strings.Split(d.scope, ","))
	for op := range ops {
		if op == "all" {
			// Perform all operations
			if err := d.backup(ctx, opts); err != nil {
				errsList = append(errsList, fmt.Errorf("failure in executing backup diagnostic operations, failed with error %v", err))
			}
		} else if op == "backup" {
			// Perform backup operation
			if err := d.backup(ctx, opts); err != nil {
				errsList = append(errsList, fmt.Errorf("failure in executing backup diagnostic operations, failed with error %v", err))
			}
		} else if op == "io" {
			// Perform IO Operation
		}
	}
	return errsList
}

// validateParams checks if the parameters provided to the OTE subcommand are valid.
func (d *Diagnose) validateParams(ctx context.Context, flagSet *flag.FlagSet) bool {
	scopeValues := map[string]bool{
		"all":    false,
		"backup": false,
		"io":     false,
	}

	scopes := strings.Split(d.scope, ",")
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if _, ok := scopeValues[scope]; !ok {
			onetime.LogMessageToFileAndConsole("Invalid flag usage, incorrect value of scope. Please check usage.")
			flagSet.Usage()
			return false
		}
		scopeValues[scope] = true
	}

	if scopeValues["backup"] || scopeValues["all"] {
		if d.paramFile == "" && d.testBucket == "" {
			onetime.LogErrorToFileAndConsole("invalid flag usage", errors.New("test bucket cannot be empty to perform backup operation"))
			return false
		}
	}
	if d.path == "" {
		log.CtxLogger(ctx).Debugw("No path for bundle provided. Setting bundle path to default", "path", bundlePath)
		d.path = bundlePath
	}
	if d.bundleName == "" {
		timestamp := time.Now().Unix()
		d.bundleName = "performance-diagnostics-" + strconv.FormatInt(timestamp, 10)
		log.CtxLogger(ctx).Debugw("No name for bundle provided. Setting bundle name using timestamp", "bundle_name", timestamp)
	}
	return true
}

func (d *Diagnose) runConfigureInstanceOTE(ctx context.Context, f *flag.FlagSet, exec commandlineexecutor.Execute, lp log.Parameters, cp *ipb.CloudProperties) subcommands.ExitStatus {
	ciOTEParams := &onetime.InternallyInvokedOTE{
		Lp:        lp,
		Cp:        cp,
		InvokedBy: d.Name(),
	}
	ci := &configureinstance.ConfigureInstance{
		Check:                  true,
		ExecuteFunc:            exec,
		OverrideHyperThreading: d.overrideHyperThreading,
		IIOTEParams:            ciOTEParams,
	}
	return ci.Execute(ctx, f)
}

// listOperations returns a map of operations to be performed.
func listOperations(ctx context.Context, operations []string) map[string]struct{} {
	ops := make(map[string]struct{})
	for _, op := range operations {
		op = strings.TrimSpace(op)
		if _, ok := ops[op]; ok {
			log.CtxLogger(ctx).Debugw("Duplicate operation already listed", "operation", op)
		}
		if op == "all" {
			return map[string]struct{}{"all": {}}
		} else if op == "backup" {
			ops[op] = struct{}{}
		} else if op == "io" {
			ops[op] = struct{}{}
		} else {
			log.CtxLogger(ctx).Debugw("Invalid operation provided", "operation", op)
		}
	}
	return ops
}

// backup performs the backup operation.
func (d *Diagnose) backup(ctx context.Context, opts *options) error {
	// Check for retention
	if err := d.checkRetention(ctx, s.NewClient, getBucket, opts.config); err != nil {
		return err
	}

	// Perform backint operation
	if err := d.runBackint(ctx, opts); err != nil {
		return err
	}
	// Perform gsutil operation

	log.CtxLogger(ctx).Info("Backup operation completed successfully")
	return nil
}

// getBucket is an abstraction of storage.ConnectToBucket() for ease of testing.
func getBucket(ctx context.Context, connectParams *storage.ConnectParameters) (attributes, bool) {
	return storage.ConnectToBucket(ctx, connectParams)
}

func (d *Diagnose) checkRetention(ctx context.Context, client storage.Client, ctb connectToBucket, config *bpb.BackintConfiguration) error {
	bucket := config.GetBucket()
	if d.testBucket != "" {
		bucket = d.testBucket
	}
	if bucket == "" {
		onetime.LogErrorToFileAndConsole("error determining bucket", fmt.Errorf("no bucket provided, either in param-file or as test-bucket"))
		return fmt.Errorf("no bucket provided, either in param-file or as test-bucket")
	}

	connectParams := &storage.ConnectParameters{
		StorageClient:    client,
		ServiceAccount:   config.GetServiceAccountKey(),
		BucketName:       bucket,
		UserAgentSuffix:  "Performance Diagnostics",
		VerifyConnection: true,
		MaxRetries:       config.GetRetries(),
		Endpoint:         config.GetClientEndpoint(),
	}

	var bucketHandle attributes
	var ok bool
	if bucketHandle, ok = ctb(ctx, connectParams); !ok {
		err := errors.New("error establishing connection to bucket, please check the logs")
		onetime.LogErrorToFileAndConsole("failed connecting to bucket", err)
		return err
	}

	attrs, err := bucketHandle.Attrs(ctx)
	if err != nil {
		log.CtxLogger(ctx).Errorw("failed to get bucket attributes, ensure the bucket exists and you have permission to access it", "error", err)
		return err
	}
	if attrs.RetentionPolicy != nil {
		err := errors.New("retention policy is set, backup operation will fail; please provide a bucket without retention policy")
		onetime.LogErrorToFileAndConsole("Error performing backup operation", err)
		return err
	}
	return nil
}

// runBackint runs the diagnose function of Backint OTE.
func (d *Diagnose) runBackint(ctx context.Context, opts *options) error {
	if err := d.setBucket(ctx, opts.fs, opts.config); err != nil {
		return err
	}

	backintParams := &backint.Backint{
		Function:  "diagnose",
		User:      "perf-diag-user",
		ParamFile: d.paramFile,
		OutFile:   path.Join(d.path, d.bundleName, "backint-output.log"),
		IIOTEParams: &onetime.InternallyInvokedOTE{
			InvokedBy: "performance-diagnostics-backint",
			Lp:        opts.lp,
			Cp:        opts.cp,
		},
	}
	if res := backintParams.Execute(ctx, &flag.FlagSet{}); res != subcommands.ExitSuccess {
		onetime.SetupOneTimeLogging(opts.lp, d.Name(), log.StringLevelToZapcore(d.logLevel))
		onetime.LogMessageToFileAndConsole("Error while executing backint")
		return fmt.Errorf("error while executing backint")
	}
	onetime.SetupOneTimeLogging(opts.lp, d.Name(), log.StringLevelToZapcore(d.logLevel))

	paths := []moveFiles{
		{
			oldPath: "/var/log/google-cloud-sap-agent/performance-diagnostics-backint.log",
			newPath: path.Join(d.path, d.bundleName, "performance-diagnostics-backint.log"),
		},
		{
			oldPath: d.paramFile,
			newPath: path.Join(d.path, d.bundleName, d.getParamFileName()),
		},
	}
	if err := addToBundle(ctx, paths, opts.fs); err != nil {
		return err
	}

	// If test-bucket is provided, temporary parameters file is created to perform
	// backint operation without modifying original parameters file.
	// Deleting this temporary file.
	if d.testBucket != "" {
		if err := opts.fs.RemoveAll(d.paramFile); err != nil {
			log.CtxLogger(ctx).Errorw("Error deleting temporary parameter file created for backint", "err", err)
			return err
		}
	}

	return nil
}

// setBucket sets the bucket in the parameter file for backint operation.
func (d *Diagnose) setBucket(ctx context.Context, fs filesystem.FileSystem, config *bpb.BackintConfiguration) error { // If testBucket is not empty, then it will overwrite the bucket provided in paramFile.
	if d.testBucket != "" {
		config.Bucket = d.testBucket

		// Creating a temporary parameter file for backint config
		content, err := protojson.Marshal(config)
		if err != nil {
			return err
		}

		d.paramFile = path.Join(d.path, d.getParamFileName())
		f, err := fs.Create(d.paramFile)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := f.Write(content); err != nil {
			return err
		}

		log.CtxLogger(ctx).Debugw("Created temporary parameter file", "paramFile", d.paramFile)
	} else {
		if config.GetBucket() == "" {
			onetime.LogErrorToFileAndConsole("error determining bucket", fmt.Errorf("no bucket provided, either in param-file or as test-bucket"))
			return fmt.Errorf("no bucket provided, either in param-file or as test-bucket")
		}
	}

	return nil
}

// addToBundle adds the files to performance diagnostics bundle.
func addToBundle(ctx context.Context, paths []moveFiles, fs filesystem.FileSystem) error {
	// onetime.LogMessageToFileAndConsole("Extracting journal CTL logs...")
	var hasErrors bool
	for _, p := range paths {
		if p.oldPath == "" || p.newPath == "" {
			log.CtxLogger(ctx).Errorw("no path provided for file", "oldPath", p.oldPath, "newPath", p.newPath)
			hasErrors = true
			continue
		}
		var err error
		var srcFD, dstFD *os.File

		if srcFD, err = fs.Open(p.oldPath); err != nil {
			return err
		}
		defer srcFD.Close()
		if dstFD, err = fs.Create(p.newPath); err != nil {
			return err
		}
		defer dstFD.Close()

		if _, err := fs.Copy(dstFD, srcFD); err != nil {
			onetime.LogErrorToFileAndConsole("error while renaming file: %v", err)
			hasErrors = true
		}
	}

	if hasErrors {
		return fmt.Errorf("error while adding files to bundle")
	}
	return nil
}

func (d *Diagnose) unmarshalBackintConfig(ctx context.Context, read ReadConfigFile) (*bpb.BackintConfiguration, error) {
	content, err := read(d.paramFile)
	if err != nil {
		log.CtxLogger(ctx).Errorw("Error reading parameters file", "err", err)
		return nil, err
	}
	if content == nil {
		err := errors.New("param file is empty")
		log.CtxLogger(ctx).Errorw("Error reading parameters file", "err", err)
		return nil, err
	}

	config, err := configuration.Unmarshal(d.paramFile, content)
	if err != nil {
		log.CtxLogger(ctx).Errorw("Error unmarshalling parameters file", "err", err)
		return nil, err
	}
	return config, nil
}

// getParamFileName extracts the name of the parameter file from the path provided.
func (d *Diagnose) getParamFileName() string {
	if d.paramFile == "" {
		return ""
	}
	parts := strings.Split(d.paramFile, "/")
	return parts[len(parts)-1]
}
