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

// Package diagnose runs all Backint functions (backup, inquire, restore, delete)
// including bucket/API connections to test and diagnose Backint issues.
package diagnose

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	store "cloud.google.com/go/storage"
	"github.com/GoogleCloudPlatform/sapagent/internal/backint/backup"
	"github.com/GoogleCloudPlatform/sapagent/internal/backint/delete"
	"github.com/GoogleCloudPlatform/sapagent/internal/backint/inquire"
	"github.com/GoogleCloudPlatform/sapagent/internal/backint/parse"
	"github.com/GoogleCloudPlatform/sapagent/internal/backint/restore"
	"github.com/GoogleCloudPlatform/sapagent/internal/configuration"
	"github.com/GoogleCloudPlatform/sapagent/internal/log"
	"github.com/GoogleCloudPlatform/sapagent/internal/usagemetrics"
	bpb "github.com/GoogleCloudPlatform/sapagent/protos/backint"
)

var (
	oneGB         = int64(1024 * 1024 * 1024)
	sixteenGB     = 16 * oneGB
	fileName1     = "/tmp/backint-diagnose-file1.txt"
	fileName2     = "/tmp/backint-diagnose-file2.txt"
	fileNotExists = "/tmp/backint-diagnose-file-not-exists.txt"
)

// diagnoseFile holds relevant info for diagnosing backup, restore, inquire, and delete functions.
type diagnoseFile struct {
	fileSize         int64
	fileName         string
	externalBackupID string
}

// executeFunc abstracts the execution of each of the Backint functions.
type executeFunc func(ctx context.Context, config *bpb.BackintConfiguration, bucketHandle *store.BucketHandle, input io.Reader, output io.Writer) bool

// removeFunc abstracts removing a file from the file system.
type removeFunc func(name string) error

// diagnoseOptions holds options for performing the requested diagnostic operation.
type diagnoseOptions struct {
	config       *bpb.BackintConfiguration
	bucketHandle *store.BucketHandle
	output       io.Writer
	files        []*diagnoseFile
	execute      executeFunc
	input        string
	want         string
}

// Execute logs information and performs the diagnostic. Returns false on failures.
func Execute(ctx context.Context, config *bpb.BackintConfiguration, bucketHandle *store.BucketHandle, output io.Writer) bool {
	log.Logger.Infow("DIAGNOSE starting", "outFile", config.GetOutputFile())
	usagemetrics.Action(usagemetrics.BackintDiagnoseStarted)
	if err := diagnose(ctx, config, bucketHandle, output); err != nil {
		log.Logger.Errorw("DIAGNOSE failed", "err", err)
		usagemetrics.Error(usagemetrics.BackintDiagnoseFailure)
		output.Write([]byte(fmt.Sprintf("\nDIAGNOSE failed: %v\n", err.Error())))
		return false
	}
	log.Logger.Infow("DIAGNOSE finished", "outFile", config.GetOutputFile())
	usagemetrics.Action(usagemetrics.BackintDiagnoseFinished)
	output.Write([]byte("\nDIAGNOSE finished\n"))
	return true
}

// diagnose runs all Backint functions (backup, inquire, restore, delete).
// Several files will be created, uploaded, queried, downloaded, and deleted.
// Results are written to the output. Any issues will return errors after
// attempting to clean up the temporary files locally and in the bucket.
func diagnose(ctx context.Context, config *bpb.BackintConfiguration, bucketHandle *store.BucketHandle, output io.Writer) error {
	files, err := createFiles(ctx, fileName1, fileName2, oneGB, sixteenGB)
	if err != nil {
		return fmt.Errorf("createFiles error: %v", err)
	}
	opts := diagnoseOptions{config: config, bucketHandle: bucketHandle, output: output, files: files}
	defer removeFiles(ctx, opts, os.Remove)

	opts.execute = backup.Execute
	if err := diagnoseBackup(ctx, opts); err != nil {
		return fmt.Errorf("backup error: %v", err)
	}
	opts.execute = inquire.Execute
	if err := diagnoseInquire(ctx, opts); err != nil {
		return fmt.Errorf("inquire error: %v", err)
	}
	opts.execute = restore.Execute
	if err := diagnoseRestore(ctx, opts); err != nil {
		return fmt.Errorf("restore error: %v", err)
	}
	opts.execute = delete.Execute
	if err := diagnoseDelete(ctx, opts); err != nil {
		return fmt.Errorf("delete error: %v", err)
	}
	return nil
}

// createFiles creates 2 files filled with '0's of different sizes.
// Issues with file operations will return errors.
func createFiles(ctx context.Context, fileName1, fileName2 string, fileSize1, fileSize2 int64) ([]*diagnoseFile, error) {
	log.Logger.Infow("Creating files for diagnostics.")
	file1, err := os.Create(fileName1)
	if err != nil {
		return nil, err
	}
	defer file1.Close()
	if err := file1.Truncate(fileSize1); err != nil {
		os.Remove(fileName1)
		return nil, err
	}

	file2, err := os.Create(fileName2)
	if err != nil {
		os.Remove(fileName1)
		return nil, err
	}
	defer file2.Close()
	if err := file2.Truncate(fileSize2); err != nil {
		os.Remove(fileName1)
		os.Remove(fileName2)
		return nil, err
	}

	// The first file is added twice to test multiple versions of the same file in the bucket.
	files := []*diagnoseFile{
		{fileSize: fileSize1, fileName: fileName1},
		{fileSize: fileSize1, fileName: fileName1},
		{fileSize: fileSize2, fileName: fileName2}}
	return files, nil
}

// removeFiles cleans up the local and bucket files used for diagnostics.
// Returns true if all files were deleted and false if there was a deletion error.
func removeFiles(ctx context.Context, opts diagnoseOptions, remove removeFunc) bool {
	log.Logger.Infow("Cleaning up files created for diagnostics.")
	allFilesDeleted := true
	for _, file := range opts.files {
		if !strings.HasPrefix(file.fileName, "/tmp/") {
			log.Logger.Errorw(`File not located in "/tmp/", cannot remove`, "file", file.fileName)
			allFilesDeleted = false
			continue
		}
		if err := remove(file.fileName); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Logger.Errorw("Failed to remove local file", "file", file.fileName, "err", err)
			allFilesDeleted = false
		}
		if opts.bucketHandle != nil {
			object := opts.config.GetUserId() + file.fileName + "/" + file.externalBackupID + ".bak"
			if err := opts.bucketHandle.Object(object).Delete(ctx); err != nil && !errors.Is(err, store.ErrObjectNotExist) {
				log.Logger.Errorw("Failed to remove bucket file", "object", object, "err", err)
				allFilesDeleted = false
			}
		}
	}
	return allFilesDeleted
}

// diagnoseBackup uploads the files to the bucket.
// The external backup IDs are saved for future diagnostics.
// Also ensure an error is present if a fake file is requested to backup.
func diagnoseBackup(ctx context.Context, opts diagnoseOptions) error {
	if len(opts.files) == 0 {
		return fmt.Errorf("no files to backup")
	}
	opts.output.Write([]byte("BACKUP:\n"))
	for _, file := range opts.files {
		opts.input = fmt.Sprintf("#FILE %q %d", file.fileName, file.fileSize)
		opts.want = "#SAVED <external_backup_id> <file_name> <size>"
		splitLines, err := performDiagnostic(ctx, opts)
		if err != nil {
			return err
		}
		file.externalBackupID = strings.Trim(splitLines[0][1], `"`)
	}

	opts.input = fmt.Sprintf("#PIPE %q %d", fileNotExists, oneGB)
	opts.want = "#ERROR <file_name>"
	if _, err := performDiagnostic(ctx, opts); err != nil {
		return err
	}
	return nil
}

// diagnoseInquire ensures all the files are present in the bucket when queried individually.
// Also check that the backups are returned in the proper order if NULL is specified.
// Lastly check that a fake filename is not found in the bucket.
func diagnoseInquire(ctx context.Context, opts diagnoseOptions) error {
	if len(opts.files) == 0 {
		return fmt.Errorf("no files to inquire")
	}
	opts.output.Write([]byte("\nINQUIRE:\n"))
	for _, file := range opts.files {
		// No version supplied, the default is to leave out creation_timestamp.
		opts.input = fmt.Sprintf("#EBID %q %q", file.externalBackupID, file.fileName)
		opts.want = "#BACKUP <external_backup_id> <file_name>"
		if _, err := performDiagnostic(ctx, opts); err != nil {
			return err
		}
	}

	// Version is supplied as >= 1.50, expecting creation_timestamp is present.
	opts.input = fmt.Sprintf(`#SOFTWAREID "backint 1.50" "Google %s %s"`+"\n#NULL %q", configuration.AgentName, configuration.AgentVersion, opts.files[0].fileName)
	opts.want = "#SOFTWAREID <backint_version> <agent_version>\n#BACKUP <external_backup_id> <file_name> <creation_timestamp>\n#BACKUP <external_backup_id> <file_name> <creation_timestamp>"
	splitLines, err := performDiagnostic(ctx, opts)
	if err != nil {
		return err
	}
	// Ensure the creation_timestamp for the first line is older than the second line.
	if splitLines[1][3] < splitLines[2][3] {
		return fmt.Errorf("inquiry files are out of order based on creation time, file1: %s, file2: %s", splitLines[1], splitLines[2])
	}

	opts.input = fmt.Sprintf("#NULL %q", fileNotExists)
	opts.want = "#NOTFOUND <file_name>"
	if _, err := performDiagnostic(ctx, opts); err != nil {
		return err
	}

	opts.input = fmt.Sprintf("#NULL %q", fileNotExists)
	opts.want = "#ERROR <file_name>"
	opts.bucketHandle = nil
	if _, err := performDiagnostic(ctx, opts); err != nil {
		return err
	}
	return nil
}

// diagnoseRestore downloads the files from the bucket.
// The download will overwrite the previously created local files.
// Also check that a fake filename is not restored from the bucket.
func diagnoseRestore(ctx context.Context, opts diagnoseOptions) error {
	if len(opts.files) == 0 {
		return fmt.Errorf("no files to restore")
	}
	opts.output.Write([]byte("\nRESTORE:\n"))
	for _, file := range opts.files {
		opts.input = fmt.Sprintf("#EBID %q %q", file.externalBackupID, file.fileName)
		opts.want = "#RESTORED <external_backup_id> <file_name>"
		if _, err := performDiagnostic(ctx, opts); err != nil {
			return err
		}
	}

	opts.input = fmt.Sprintf("#NULL %q", opts.files[0].fileName)
	opts.want = "#RESTORED <external_backup_id> <file_name>"
	if _, err := performDiagnostic(ctx, opts); err != nil {
		return err
	}

	opts.input = fmt.Sprintf("#NULL %q", fileNotExists)
	opts.want = "#NOTFOUND <file_name>"
	if _, err := performDiagnostic(ctx, opts); err != nil {
		return err
	}

	opts.input = fmt.Sprintf("#NULL %q", fileNotExists)
	opts.want = "#ERROR <file_name>"
	opts.bucketHandle = nil
	if _, err := performDiagnostic(ctx, opts); err != nil {
		return err
	}
	return nil
}

// diagnoseDelete removes all files from the bucket and
// then ensures they are not found on a subsequent request.
func diagnoseDelete(ctx context.Context, opts diagnoseOptions) error {
	if len(opts.files) == 0 {
		return fmt.Errorf("no files to delete")
	}
	opts.output.Write([]byte("\nDELETE:\n"))
	for _, file := range opts.files {
		opts.input = fmt.Sprintf("#EBID %q %q", file.externalBackupID, file.fileName)
		opts.want = "#DELETED <external_backup_id> <file_name>"
		if _, err := performDiagnostic(ctx, opts); err != nil {
			return err
		}
	}

	for _, file := range opts.files {
		opts.input = fmt.Sprintf("#EBID %q %q", file.externalBackupID, file.fileName)
		opts.want = "#NOTFOUND <external_backup_id> <file_name>"
		if _, err := performDiagnostic(ctx, opts); err != nil {
			return err
		}
	}

	opts.input = fmt.Sprintf("#EBID %q %q", "12345", fileNotExists)
	opts.want = "#ERROR <external_backup_id> <file_name>"
	opts.bucketHandle = nil
	if _, err := performDiagnostic(ctx, opts); err != nil {
		return err
	}
	return nil
}

// performDiagnostic executes the desired Backint function and verifies the output lines.
// The return is 2D slice where each output line is a row and each line's split value is a column.
func performDiagnostic(ctx context.Context, opts diagnoseOptions) ([][]string, error) {
	in := bytes.NewBufferString(opts.input)
	out := bytes.NewBufferString("")
	if ok := opts.execute(ctx, opts.config, opts.bucketHandle, in, out); !ok {
		return nil, fmt.Errorf("failed to execute: %s", opts.input)
	}
	opts.output.Write(out.Bytes())

	// Remove the final newline before splitting the lines on newlines.
	outTrim := strings.Trim(out.String(), "\n")
	lines := strings.Split(outTrim, "\n")
	wantLines := strings.Split(opts.want, "\n")
	if len(lines) != len(wantLines) {
		return nil, fmt.Errorf("unexpected number of output lines for input: %s, got: %d, want: %d", opts.input, len(lines), len(wantLines))
	}

	var splitLines [][]string
	for i, line := range lines {
		wantSplit := parse.Split(wantLines[i])
		if !strings.HasPrefix(line, wantSplit[0]) {
			return nil, fmt.Errorf("malformed output line for input: %s, got: %s, want prefix: %s", opts.input, line, wantSplit[0])
		}
		s := parse.Split(line)
		if len(s) != len(wantSplit) {
			return nil, fmt.Errorf("malformed output line for input: %s, got: %s, want format: %s", opts.input, line, wantLines[i])
		}
		splitLines = append(splitLines, s)
	}
	return splitLines, nil
}
