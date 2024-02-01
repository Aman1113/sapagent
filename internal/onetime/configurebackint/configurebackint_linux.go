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

// Package configurebackint implements OTE mode for editing JSON configuration
// files for Backint and migrating to JSON from the old agent's TXT
// configuration. TXT configuration files are never updated from this OTE.
package configurebackint

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"strings"

	"flag"
	wpb "google.golang.org/protobuf/types/known/wrapperspb"
	"golang.org/x/sys/unix"
	"google.golang.org/protobuf/encoding/protojson"
	"github.com/google/subcommands"
	"github.com/GoogleCloudPlatform/sapagent/internal/backint/configuration"
	"github.com/GoogleCloudPlatform/sapagent/internal/onetime"
	"github.com/GoogleCloudPlatform/sapagent/internal/usagemetrics"
	bpb "github.com/GoogleCloudPlatform/sapagent/protos/backint"
	"github.com/GoogleCloudPlatform/sapagent/shared/log"
)

type (
	// writeFileFunc provides a testable replacement for os.WriteFile.
	writeFileFunc func(string, []byte, os.FileMode) error

	// statFunc provides a testable replacement for unix.Stat.
	statFunc func(string, *unix.Stat_t) error

	// readFileFunc provides a testable replacement for os.ReadFile.
	readFileFunc func(string) ([]byte, error)

	// chmodFunc provides a testable replacement for os.Chmod.
	chmodFunc func(string, os.FileMode) error

	// chownFunc provides a testable replacement for os.Chown.
	chownFunc func(string, int, int) error
)

// ConfigureBackint has args for configurebackint subcommands.
type ConfigureBackint struct {
	fileName      string
	help, version bool

	bucket, recoveryBucket, folderPrefix, recoveryFolderPrefix           string
	encryptionKey, kmsKey, logLevel, serviceAccountKey, clientEndpoint   string
	parallelStreams, threads, retries, bufferSizeMb, rateLimitMb         int64
	logDelaySec, fileReadTimeoutMs, retryBackoffInitial, retryBackoffMax int64
	retryBackoffMultiplier                                               float64
	compress, logToCloud                                                 bool

	writeFile writeFileFunc
	stat      statFunc
	readFile  readFileFunc
	chmod     chmodFunc
	chown     chownFunc
}

// Name implements the subcommand interface for configurebackint.
func (*ConfigureBackint) Name() string { return "configurebackint" }

// Synopsis implements the subcommand interface for configurebackint.
func (*ConfigureBackint) Synopsis() string {
	return "edit Backint JSON configuration files and migrate from legacy agent's TXT configuration files"
}

// Usage implements the subcommand interface for configurebackint.
func (*ConfigureBackint) Usage() string {
	return `Usage: configurebackint -f=<path/to/backint/parameters.json|path/to/backint/parameters.txt>

	[-bucket=<bucket-name>] [-recovery-bucket=<bucket-name>] [-log-to-cloud=<false>] [-log-level=<"INFO">]
	[-compress=<false>] [-encryption-key=</path/to/key/file>] [-kms-key=</path/to/key/file>]
	[-retries=<5>] [-parallel-streams=<1>] [-rate-limit-mb=<0>] [-service-account-key=</path/to/key/file>]
	[-threads=<64>] [-file-read-timeout-ms=<60000>]	[-buffer-size-mb=<100>]
	[-retry-backoff-initial=<10>]	[-retry-backoff-max=<300>] [-retry-backoff-multiplier=<2>]
	[-log-delay-sec=<60>]	[-client-endpoint=<"custom.endpoint.com">]
	[-folder-prefix=<"prefix/path">] [-recovery-folder-prefix=<"prefix/path">]
	[-h] [-v]` + "\n"
}

// SetFlags implements the subcommand interface for configurebackint.
func (c *ConfigureBackint) SetFlags(fs *flag.FlagSet) {
	fs.StringVar(&c.fileName, "f", "", "Path to the JSON or TXT configuration file")
	fs.BoolVar(&c.help, "h", false, "Displays help")
	fs.BoolVar(&c.version, "v", false, "Displays the current version of the agent")

	// Using underscores for config parameters to match the proto values.
	fs.StringVar(&c.bucket, "bucket", "", "Specify the name of the Cloud Storage bucket that the Google Cloud's Agent for SAP writes to and reads from.")
	fs.StringVar(&c.recoveryBucket, "recovery-bucket", "", "Specify the name of the Cloud Storage bucket that the Google Cloud's Agent for SAP writes to and reads from for RESTORE operations.")
	fs.BoolVar(&c.logToCloud, "log-to-cloud", false, "To redirect the Backint related logs of Google Cloud's Agent for SAP, to Cloud Logging, specify true.")
	fs.StringVar(&c.logLevel, "log-level", "", "Specify the logging level for the Backint feature of Google Cloud's Agent for SAP.")
	fs.BoolVar(&c.compress, "compress", false, "Specify whether or not Google Cloud's Agent for SAP is to enable compression while writing backups to the Cloud Storage bucket.")
	fs.StringVar(&c.encryptionKey, "encryption-key", "", "Specify the path to the customer-supplied encryption key that you've configured your Cloud Storage bucket to use to encrypt backups.")
	fs.StringVar(&c.kmsKey, "kms-key", "", "Specify the path to the customer-managed encryption key that you've configured your Cloud Storage bucket to use to encrypt backups.")
	fs.Int64Var(&c.retries, "retries", 0, "Specifies the maximum number of times that Google Cloud's Agent for SAP retries a failed attempt to read or write to Cloud Storage.")
	fs.Int64Var(&c.parallelStreams, "parallel-streams", 0, "Specify to enable parallel upload and specifies the maximum number of parallel upload streams that Google Cloud's Agent for SAP can use.")
	fs.Int64Var(&c.rateLimitMb, "rate-limit-mb", 0, "Specify the upper limit, in MB, for the outbound network bandwidth of Compute Engine during backup or restore operations.")
	fs.StringVar(&c.serviceAccountKey, "service-account-key", "", "If Google Cloud's Agent for SAP is not running on a Compute Engine VM, then specify the fully-qualified path to the JSON-encoded Google Cloud service account.")
	fs.Int64Var(&c.threads, "threads", 0, "Specify the number of worker threads.")
	fs.Int64Var(&c.fileReadTimeoutMs, "file-read-timeout-ms", 0, "Specify the maximum amount of time, in milliseconds, that Google Cloud's Agent for SAP waits to open the backup file.")
	fs.Int64Var(&c.bufferSizeMb, "buffer-size-mb", 0, "Specify this parameter to control the size of HTTPS requests to Cloud Storage during backup or restore operations.")
	fs.Int64Var(&c.retryBackoffInitial, "retry-backoff-initial", 0, "Specify the initial value, in seconds, for the retry period used in the exponential backoff network retries.")
	fs.Int64Var(&c.retryBackoffMax, "retry-backoff-max", 0, "Specify the maximum value, in seconds, for the retry period used in the exponential backoff network retries.")
	fs.Float64Var(&c.retryBackoffMultiplier, "retry-backoff-multiplier", 0, "Specify the multiplier for the retry period used in the exponential backoff network retries.")
	fs.Int64Var(&c.logDelaySec, "log-delay-sec", 0, "Specify the logging delay, in seconds, for progress updates during reads and writes to the Cloud Storage bucket.")
	fs.StringVar(&c.clientEndpoint, "client-endpoint", "", "Specify the endpoint of the Cloud Storage client.")
	fs.StringVar(&c.folderPrefix, "folder-prefix", "", "Specify the folder prefix of the Cloud Storage bucket that the Google Cloud's Agent for SAP writes to and reads from.")
	fs.StringVar(&c.recoveryFolderPrefix, "recovery-folder-prefix", "", "Specify the folder prefix of the Cloud Storage bucket that the Google Cloud's Agent for SAP writes to and reads from for RESTORE operations.")
}

// Execute implements the subcommand interface for configurebackint.
func (c *ConfigureBackint) Execute(ctx context.Context, f *flag.FlagSet, args ...any) subcommands.ExitStatus {
	if c.help {
		return onetime.HelpCommand(f)
	}
	if c.version {
		onetime.PrintAgentVersion()
		return subcommands.ExitSuccess
	}
	if len(args) < 2 {
		log.CtxLogger(ctx).Errorf("Not enough args for Execute(). Want: 3, Got: %d", len(args))
		return subcommands.ExitUsageError
	}
	lp, ok := args[1].(log.Parameters)
	if !ok {
		log.CtxLogger(ctx).Errorf("Unable to assert args[1] of type %T to log.Parameters.", args[1])
		return subcommands.ExitUsageError
	}
	onetime.SetupOneTimeLogging(lp, c.Name(), log.StringLevelToZapcore("info"))

	if c.fileName == "" {
		fmt.Printf("-f must be specified.\n%s\n", c.Usage())
		log.CtxLogger(ctx).Errorf("-f must be specified")
		return subcommands.ExitUsageError
	}

	c.writeFile = os.WriteFile
	c.stat = unix.Stat
	c.readFile = os.ReadFile
	c.chmod = os.Chmod
	c.chown = os.Chown
	if err := c.configureBackintHandler(ctx, f); err != nil {
		fmt.Println("Backint configuration: FAILED, detailed logs are at /var/log/google-cloud-sap-agent/configurebackint.log")
		log.CtxLogger(ctx).Errorw("ConfigureBackint failed", "fileName", c.fileName, "err", err)
		usagemetrics.Error(usagemetrics.ConfigureBackintFailure)
		return subcommands.ExitFailure
	}
	return subcommands.ExitSuccess
}

// configureBackintHandler edits JSON configuration files and migrates legacy
// TXT configuration files from the old agent.
func (c *ConfigureBackint) configureBackintHandler(ctx context.Context, f *flag.FlagSet) error {
	log.CtxLogger(ctx).Info("ConfigureBackint starting")
	usagemetrics.Action(usagemetrics.ConfigureBackintStarted)
	var stat unix.Stat_t
	if err := c.stat(c.fileName, &stat); os.IsNotExist(err) {
		return fmt.Errorf("backint configuration file not found: %s", c.fileName)
	} else if err != nil {
		return fmt.Errorf("unable to stat backint configuration file: %s, err: %v", c.fileName, err)
	}
	log.CtxLogger(ctx).Infow("Configuration file info", "fileName", c.fileName, "uid", int(stat.Uid), "gid", int(stat.Gid))

	config, err := c.unmarshalConfigFile(ctx)
	if err != nil {
		return err
	}
	config = c.updateConfig(ctx, config, f)
	if err := c.createAndChownFile(ctx, c.fileName, config, 0640, int(stat.Uid), int(stat.Gid)); err != nil {
		return err
	}

	fmt.Println("Backint configuration: SUCCESS, detailed logs are at /var/log/google-cloud-sap-agent/configurebackint.log")
	log.CtxLogger(ctx).Info("ConfigureBackint succeeded")
	usagemetrics.Action(usagemetrics.ConfigureBackintFinished)
	return nil
}

// unmarshalConfigFile reads the config file (JSON or TXT) and
// unmarshals it using the Backint configuration package.
func (c *ConfigureBackint) unmarshalConfigFile(ctx context.Context) (*bpb.BackintConfiguration, error) {
	content, err := c.readFile(c.fileName)
	if err != nil {
		return nil, fmt.Errorf("unable to read backint configuration file: %s, err: %v", c.fileName, err)
	}
	if len(content) == 0 {
		return nil, fmt.Errorf("empty backint configuration file: %s", c.fileName)
	}
	config, err := configuration.Unmarshal(c.fileName, content)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal backint configuration file: %s, err: %v", c.fileName, err)
	}
	return config, nil
}

// updateConfig updates configuration options based on the OTE arguments.
func (c *ConfigureBackint) updateConfig(ctx context.Context, config *bpb.BackintConfiguration, f *flag.FlagSet) *bpb.BackintConfiguration {
	// Visit() visits only those flags that have been set. This ensures empty
	// strings and other zero values will only be updated if explicitly set.
	f.Visit(func(f *flag.Flag) {
		log.CtxLogger(ctx).Infow("Updating configuration", "flagName", f.Name, "flagValue", f.Value.String())
		switch f.Name {
		case "bucket":
			config.Bucket = c.bucket
		case "recovery-bucket":
			config.RecoveryBucket = c.recoveryBucket
		case "log-to-cloud":
			config.LogToCloud = &wpb.BoolValue{Value: c.logToCloud}
		case "log-level":
			config.LogLevel = bpb.LogLevel(bpb.LogLevel_value[strings.ToUpper(c.logLevel)])
		case "compress":
			config.Compress = c.compress
		case "encryption-key":
			config.EncryptionKey = c.encryptionKey
		case "kms-key":
			config.KmsKey = c.kmsKey
		case "retries":
			config.Retries = c.retries
		case "parallel-streams":
			config.ParallelStreams = c.parallelStreams
		case "rate-limit-mb":
			config.RateLimitMb = c.rateLimitMb
		case "service-account-key":
			config.ServiceAccountKey = c.serviceAccountKey
		case "threads":
			config.Threads = c.threads
		case "file-read-timeout-ms":
			config.FileReadTimeoutMs = c.fileReadTimeoutMs
		case "buffer-size-mb":
			config.BufferSizeMb = c.bufferSizeMb
		case "retry-backoff-initial":
			config.RetryBackoffInitial = c.retryBackoffInitial
		case "retry-backoff-max":
			config.RetryBackoffMax = c.retryBackoffMax
		case "retry-backoff-multiplier":
			config.RetryBackoffMultiplier = float32(c.retryBackoffMultiplier)
		case "log-delay-sec":
			config.LogDelaySec = c.logDelaySec
		case "client-endpoint":
			config.ClientEndpoint = c.clientEndpoint
		case "folder-prefix":
			config.FolderPrefix = c.folderPrefix
		case "recovery-folder-prefix":
			config.RecoveryFolderPrefix = c.recoveryFolderPrefix
		}
	})
	return config
}

// createAndChownFile marshals the config data, creates the file if it does not
// exist, writes data, updates permissions, and chowns to the user and group.
func (c *ConfigureBackint) createAndChownFile(ctx context.Context, file string, config *bpb.BackintConfiguration, permissions os.FileMode, uid, gid int) error {
	if strings.HasSuffix(file, ".txt") {
		file = strings.TrimSuffix(file, ".txt") + ".json"
		log.CtxLogger(ctx).Infow("Converting TXT input file to JSON output", "fileName", file)
	}
	log.CtxLogger(ctx).Infow("Writing configuration file", "fileName", file, "config", config)
	configData, err := protojson.MarshalOptions{Indent: "  ", UseProtoNames: true}.Marshal(config)
	if err != nil {
		return fmt.Errorf("unable to marshal config, err: %v", err)
	}
	if err := c.writeFile(file, configData, permissions); err != nil {
		return fmt.Errorf("unable to write file: %s, err: %v", file, err)
	}
	if err := c.chmod(file, permissions); err != nil {
		return fmt.Errorf("unable to chmod file: %s. err: %v", file, err)
	}
	if err := c.chown(file, uid, gid); err != nil {
		return fmt.Errorf("unable to chown file: %s, uid: %d, gid: %d, err: %v", file, uid, gid, err)
	}
	return nil
}
