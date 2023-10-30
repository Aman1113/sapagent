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

// Package installbackint implements OTE mode for installing Backint files
// necessary for SAP HANA, and migrating from the old Backint agent.
package installbackint

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"flag"
	wpb "google.golang.org/protobuf/types/known/wrapperspb"
	"golang.org/x/sys/unix"
	"google.golang.org/protobuf/encoding/protojson"
	"github.com/google/subcommands"
	"github.com/GoogleCloudPlatform/sapagent/internal/onetime"
	"github.com/GoogleCloudPlatform/sapagent/internal/usagemetrics"
	bpb "github.com/GoogleCloudPlatform/sapagent/protos/backint"
	"github.com/GoogleCloudPlatform/sapagent/shared/log"
)

//go:embed hdbbackint.sh
var hdbbackintScript []byte

// DateTimeMinutes is a reference for timestamps to minute granularity.
const DateTimeMinutes = "2006-01-02T15:04"

type (
	// mkdirFunc provides a testable replacement for os.MkdirAll.
	mkdirFunc func(string, os.FileMode) error

	// writeFileFunc provides a testable replacement for os.WriteFile.
	writeFileFunc func(string, []byte, os.FileMode) error

	// symlinkFunc provides a testable replacement for os.Symlink.
	symlinkFunc func(string, string) error

	// statFunc provides a testable replacement for unix.Stat.
	statFunc func(string, *unix.Stat_t) error

	// renameFunc provides a testable replacement for os.Rename.
	renameFunc func(string, string) error

	// globFunc provides a testable replacement for filepath.Glob.
	globFunc func(string) ([]string, error)

	// readFileFunc provides a testable replacement for os.ReadFile.
	readFileFunc func(string) ([]byte, error)

	// chmodFunc provides a testable replacement for os.Chmod.
	chmodFunc func(string, os.FileMode) error

	// chownFunc provides a testable replacement for os.Chown.
	chownFunc func(string, int, int) error
)

// InstallBackint has args for installbackint subcommands.
type InstallBackint struct {
	sid, logLevel string
	help, version bool

	mkdir     mkdirFunc
	writeFile writeFileFunc
	symlink   symlinkFunc
	stat      statFunc
	rename    renameFunc
	glob      globFunc
	readFile  readFileFunc
	chmod     chmodFunc
	chown     chownFunc
}

// Name implements the subcommand interface for installbackint.
func (*InstallBackint) Name() string { return "installbackint" }

// Synopsis implements the subcommand interface for installbackint.
func (*InstallBackint) Synopsis() string {
	return "install Backint and migrate from Backint agent for SAP HANA"
}

// Usage implements the subcommand interface for installbackint.
func (*InstallBackint) Usage() string {
	return `installbackint [-sid=<sap-system-identification>]
	[-h] [-v] [loglevel=<debug|info|warn|error>]
	`
}

// SetFlags implements the subcommand interface for installbackint.
func (b *InstallBackint) SetFlags(fs *flag.FlagSet) {
	fs.StringVar(&b.sid, "sid", "", "SAP System Identification, defaults to $SAPSYSTEMNAME")
	fs.BoolVar(&b.help, "h", false, "Displays help")
	fs.BoolVar(&b.version, "v", false, "Displays the current version of the agent")
	fs.StringVar(&b.logLevel, "loglevel", "info", "Sets the logging level")
}

// Execute implements the subcommand interface for installbackint.
func (b *InstallBackint) Execute(ctx context.Context, f *flag.FlagSet, args ...any) subcommands.ExitStatus {
	if len(args) < 2 {
		log.CtxLogger(ctx).Errorf("Not enough args for Execute(). Want: 3, Got: %d", len(args))
		return subcommands.ExitUsageError
	}
	lp, ok := args[1].(log.Parameters)
	if !ok {
		log.CtxLogger(ctx).Errorf("Unable to assert args[1] of type %T to log.Parameters.", args[1])
		return subcommands.ExitUsageError
	}
	if b.help {
		f.Usage()
		return subcommands.ExitSuccess
	}
	if b.version {
		onetime.PrintAgentVersion()
		return subcommands.ExitSuccess
	}
	onetime.SetupOneTimeLogging(lp, b.Name(), log.StringLevelToZapcore(b.logLevel))

	if b.sid == "" {
		b.sid = os.Getenv("SAPSYSTEMNAME")
		log.CtxLogger(ctx).Warnf("sid defaulted to $SAPSYSTEMNAME: %s", b.sid)
		if b.sid == "" {
			log.CtxLogger(ctx).Errorf("sid is not defined. Set the sid command line argument, or ensure $SAPSYSTEMNAME is set. Usage:" + b.Usage())
			return subcommands.ExitUsageError
		}
	}

	b.mkdir = os.MkdirAll
	b.writeFile = os.WriteFile
	b.symlink = os.Symlink
	b.stat = unix.Stat
	b.rename = os.Rename
	b.glob = filepath.Glob
	b.readFile = os.ReadFile
	b.chmod = os.Chmod
	b.chown = os.Chown
	if err := b.installBackintHandler(ctx, fmt.Sprintf("/usr/sap/%s/SYS/global/hdb/opt", b.sid)); err != nil {
		fmt.Println("Backint installation: FAILED, detailed logs are at /var/log/google-cloud-sap-agent/installbackint.log")
		log.CtxLogger(ctx).Errorw("InstallBackint failed", "sid", b.sid, "err", err)
		usagemetrics.Error(usagemetrics.InstallBackintFailure)
		return subcommands.ExitFailure
	}
	return subcommands.ExitSuccess
}

// installBackintHandler creates directories, files, and symlinks
// in order to execute Backint from SAP HANA for the specified sid.
func (b *InstallBackint) installBackintHandler(ctx context.Context, baseInstallDir string) error {
	log.CtxLogger(ctx).Info("InstallBackint starting")
	usagemetrics.Action(usagemetrics.InstallBackintStarted)
	var stat unix.Stat_t
	if err := b.stat(baseInstallDir, &stat); err != nil {
		return fmt.Errorf("unable to stat base install directory: %s, ensure the sid is correct. err: %v", baseInstallDir, err)
	}
	log.CtxLogger(ctx).Infow("Base directory info", "baseInstallDir", baseInstallDir, "uid", stat.Uid, "gid", stat.Gid)
	if err := b.migrateOldAgent(ctx, baseInstallDir, int(stat.Uid), int(stat.Gid)); err != nil {
		return fmt.Errorf("unable to migrate old agent. err: %v", err)
	}

	// Ensure we don't trip the Kokoro replace_func by separating the strings.
	backintInstallDir := baseInstallDir + "/backint" + "/backint-gcs"
	log.CtxLogger(ctx).Infow("Creating Backint directories", "backintInstallDir", backintInstallDir, "hdbconfigDir", baseInstallDir+"/hdbconfig")
	// Create /backint first so permissions are set for the /backint-gcs subdir.
	if err := b.createAndChownDir(ctx, baseInstallDir+"/backint", int(stat.Uid), int(stat.Gid)); err != nil {
		return err
	}
	if err := b.createAndChownDir(ctx, backintInstallDir, int(stat.Uid), int(stat.Gid)); err != nil {
		return err
	}
	if err := b.createAndChownDir(ctx, baseInstallDir+"/hdbconfig", int(stat.Uid), int(stat.Gid)); err != nil {
		return err
	}

	backintPath := backintInstallDir + "/backint"
	parameterPath := backintInstallDir + "/parameters.json"
	log.CtxLogger(ctx).Infow("Creating Backint files", "backintPath", backintPath, "parameterPath", parameterPath)
	if err := b.writeFile(backintPath, hdbbackintScript, os.ModePerm); err != nil {
		return fmt.Errorf("unable to write backint script: %s. err: %v", backintPath, err)
	}
	config := &bpb.BackintConfiguration{Bucket: "<GCS Bucket Name>", LogToCloud: wpb.Bool(true)}
	configData, err := protojson.MarshalOptions{Indent: "  ", UseProtoNames: true}.Marshal(config)
	if err != nil {
		return fmt.Errorf("unable to marshal config, err: %v", err)
	}
	if err := b.writeFile(parameterPath, configData, 0666); err != nil {
		return fmt.Errorf("unable to write parameters.json file: %s. err: %v", parameterPath, err)
	}
	if err := b.chmod(parameterPath, 0666); err != nil {
		return fmt.Errorf("unable to chmod parameters.json file: %s. err: %v", parameterPath, err)
	}

	backintSymlink := baseInstallDir + "/hdbbackint"
	parameterSymlink := baseInstallDir + "/hdbconfig/parameters.json"
	logSymlink := backintInstallDir + "/logs"
	logPath := "/var/log/google-cloud-sap-agent/"
	log.CtxLogger(ctx).Infow("Creating Backint symlinks", "backintSymlink", backintSymlink, "parameterSymlink", parameterSymlink, "logSymlink", logSymlink)
	os.Remove(backintSymlink)
	os.Remove(parameterSymlink)
	os.Remove(logSymlink)
	if err := b.symlink(backintPath, backintSymlink); err != nil {
		return fmt.Errorf("unable to create hdbbackint symlink: %s for: %s. err: %v", backintSymlink, backintPath, err)
	}
	if err := b.symlink(parameterPath, parameterSymlink); err != nil {
		return fmt.Errorf("unable to create parameters.json symlink: %s for %s. err: %v", parameterSymlink, parameterPath, err)
	}
	if err := b.symlink(logPath, logSymlink); err != nil {
		return fmt.Errorf("unable to create log symlink: %s for %s. err: %v", logSymlink, logPath, err)
	}

	fmt.Println("Backint installation: SUCCESS, detailed logs are at /var/log/google-cloud-sap-agent/installbackint.log")
	log.CtxLogger(ctx).Info("InstallBackint succeeded")
	usagemetrics.Action(usagemetrics.InstallBackintFinished)
	return nil
}

// migrateOldAgent moves the backint-gcs folder to backint-gcs-old-<timestamp>
// if it contains the old agent code (a jre directory is present).If migrating,
// all parameter.txt files are then copied to the backint-gcs folder.
func (b *InstallBackint) migrateOldAgent(ctx context.Context, baseInstallDir string, uid, gid int) error {
	// Ensure we don't trip the Kokoro replace_func by separating the strings.
	backintInstallDir := baseInstallDir + "/backint" + "/backint-gcs"
	backintOldDir := baseInstallDir + "/backint" + "/backint-gcs-old-" + time.Now().Format(DateTimeMinutes)
	jreInstallDir := backintInstallDir + "/jre"

	if err := b.stat(jreInstallDir, &unix.Stat_t{}); os.IsNotExist(err) {
		log.CtxLogger(ctx).Infow("Old Backint agent not found, skipping migration", "jreInstallDir", jreInstallDir)
		return nil
	} else if err != nil {
		return fmt.Errorf("unable to stat jre install directory: %s, err: %v", jreInstallDir, err)
	}

	log.CtxLogger(ctx).Infow("Old Backint agent found, migrating files", "oldpath", backintInstallDir, "newpath", backintOldDir)
	if err := b.rename(backintInstallDir, backintOldDir); err != nil {
		return fmt.Errorf("unable to move old backint install directory, oldpath: %s newpath: %s, err: %v", backintInstallDir, backintOldDir, err)
	}
	// Create /backint first so permissions are set for the /backint-gcs subdir.
	if err := b.createAndChownDir(ctx, baseInstallDir+"/backint", uid, gid); err != nil {
		return err
	}
	if err := b.createAndChownDir(ctx, backintInstallDir, uid, gid); err != nil {
		return err
	}
	if err := b.createAndChownDir(ctx, backintOldDir, uid, gid); err != nil {
		return err
	}

	txtFiles, err := b.glob(backintOldDir + "/*.txt")
	if err != nil {
		return fmt.Errorf("unable to glob .txt files: %s, err: %v", backintOldDir+"/*.txt", err)
	}
	for _, fileName := range txtFiles {
		if strings.HasSuffix(fileName, "VERSION.txt") {
			continue
		}
		data, err := b.readFile(fileName)
		if err != nil {
			return fmt.Errorf("unable to read parameter file: %s, err: %v", fileName, err)
		}
		destination := backintInstallDir + "/" + filepath.Base(fileName)
		if err := b.writeFile(destination, data, 0666); err != nil {
			return fmt.Errorf("unable to write parameter file: %s, err: %v", destination, err)
		}
		if err := b.chmod(destination, 0666); err != nil {
			return fmt.Errorf("unable to chmod parameters file: %s. err: %v", destination, err)
		}
	}
	log.CtxLogger(ctx).Infow("Successfully migrated old agent")
	return nil
}

// createAndChownDir creates the directory if it does not exist
// and chowns to the user and group.
func (b *InstallBackint) createAndChownDir(ctx context.Context, dir string, uid, gid int) error {
	if err := b.mkdir(dir, os.ModePerm); err != nil {
		return fmt.Errorf("unable to create directory: %s. err: %v", dir, err)
	}
	if err := b.chown(dir, uid, gid); err != nil {
		return fmt.Errorf("unable to chown directory: %s, uid: %d, gid: %d, err: %v", dir, uid, gid, err)
	}
	return nil
}
