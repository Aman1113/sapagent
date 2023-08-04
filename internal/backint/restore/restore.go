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

// Package restore downloads Backint backups from a GCS bucket.
package restore

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	store "cloud.google.com/go/storage"
	"github.com/gammazero/workerpool"
	"github.com/GoogleCloudPlatform/sapagent/internal/backint/parse"
	"github.com/GoogleCloudPlatform/sapagent/internal/storage"
	"github.com/GoogleCloudPlatform/sapagent/internal/usagemetrics"
	bpb "github.com/GoogleCloudPlatform/sapagent/protos/backint"
	"google3/third_party/sapagent/shared/log/log"
)

// Execute logs information and performs the requested restore. Returns false on failures.
func Execute(ctx context.Context, config *bpb.BackintConfiguration, bucketHandle *store.BucketHandle, input io.Reader, output io.Writer) bool {
	log.Logger.Infow("RESTORE starting", "inFile", config.GetInputFile(), "outFile", config.GetOutputFile())
	usagemetrics.Action(usagemetrics.BackintRestoreStarted)
	if err := restore(ctx, config, bucketHandle, input, output); err != nil {
		log.Logger.Errorw("RESTORE failed", "err", err)
		usagemetrics.Error(usagemetrics.BackintRestoreFailure)
		return false
	}
	log.Logger.Infow("RESTORE finished", "inFile", config.GetInputFile(), "outFile", config.GetOutputFile())
	usagemetrics.Action(usagemetrics.BackintRestoreFinished)
	return true
}

// restore downloads files from the bucket based on each line of the input. Results for each
// restore are written to the output. Issues with file operations will return errors.
func restore(ctx context.Context, config *bpb.BackintConfiguration, bucketHandle *store.BucketHandle, input io.Reader, output io.Writer) error {
	wp := workerpool.New(int(config.GetThreads()))
	mu := &sync.Mutex{}
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		line := scanner.Text()
		log.Logger.Infow("Executing restore input", "line", line)
		if strings.HasPrefix(line, "#SOFTWAREID") {
			if _, err := parse.WriteSoftwareVersion(line, output); err != nil {
				return err
			}
		} else if strings.HasPrefix(line, "#NULL") {
			s := parse.Split(line)
			if len(s) < 2 {
				return fmt.Errorf("malformed restore input line, got: %s, want: #NULL <file_name> [<dest_name>]", line)
			}
			fileName := s[1]
			destName := fileName
			// Destination is an optional parameter.
			if len(s) > 2 {
				destName = s[2]
			}
			wp.Submit(func() {
				out := restoreFile(ctx, config, bucketHandle, io.Copy, fileName, destName, "")
				mu.Lock()
				defer mu.Unlock()
				output.Write(out)
			})
		} else if strings.HasPrefix(line, "#EBID") {
			s := parse.Split(line)
			if len(s) < 3 {
				return fmt.Errorf("malformed restore input line, got: %s, want: #EBID <external_backup_id> <file_name> [<dest_name>]", line)
			}
			externalBackupID := parse.TrimAndClean(s[1])
			fileName := s[2]
			destName := fileName
			// Destination is an optional parameter.
			if len(s) > 3 {
				destName = s[3]
			}
			wp.Submit(func() {
				out := restoreFile(ctx, config, bucketHandle, io.Copy, fileName, destName, externalBackupID)
				mu.Lock()
				defer mu.Unlock()
				output.Write(out)
			})
		} else {
			log.Logger.Infow("Unknown prefix encountered, treated as a comment", "line", line)
		}
	}
	wp.StopWait()
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

// restoreFile queries the bucket to see if the backup exists.
// If externalBackupID is not specified, the latest backup for fileName is used.
// If found, the file is downloaded and saved to destName.
func restoreFile(ctx context.Context, config *bpb.BackintConfiguration, bucketHandle *store.BucketHandle, copier storage.IOFileCopier, fileName, destName, externalBackupID string) []byte {
	prefix := config.GetUserId() + parse.TrimAndClean(fileName)
	if externalBackupID != "" {
		prefix += fmt.Sprintf("/%s.bak", externalBackupID)
	}

	log.Logger.Infow("Restoring file", "userID", config.GetUserId(), "fileName", fileName, "destName", destName, "prefix", prefix, "externalBackupID", externalBackupID)
	objects, err := storage.ListObjects(ctx, bucketHandle, prefix)
	if err != nil {
		log.Logger.Errorw("Error listing objects", "fileName", fileName, "prefix", prefix, "err", err, "externalBackupID", externalBackupID)
		return []byte(fmt.Sprintf("#ERROR %s\n", fileName))
	} else if len(objects) == 0 {
		log.Logger.Warnw("No objects found", "fileName", fileName, "prefix", prefix, "externalBackupID", externalBackupID)
		return []byte(fmt.Sprintf("#NOTFOUND %s\n", fileName))
	}
	// The objects will be sorted by creation date, latest first.
	// Thus, the requested restore will always be the first in the objects slice.
	object := objects[0]

	// From the specification, files are created by Backint and pipes are created by the database.
	log.Logger.Infow("Restoring object from bucket", "fileName", fileName, "prefix", prefix, "externalBackupID", externalBackupID, "obj", object.Name, "fileType", object.Metadata["fileType"], "destName", destName, "objSize", object.Size)
	var destFile *os.File
	if object.Metadata["X-Backup-Type"] == "FILE" {
		destFile, err = os.Create(parse.TrimAndClean(destName))
	} else {
		destFile, err = os.OpenFile(parse.TrimAndClean(destName), os.O_RDWR, 0)
	}
	if err != nil {
		log.Logger.Errorw("Error opening dest file", "destName", destName, "err", err, "fileType", object.Metadata["fileType"])
		return []byte(fmt.Sprintf("#ERROR %s\n", fileName))
	}
	defer destFile.Close()
	if fileInfo, err := destFile.Stat(); err != nil || fileInfo.Mode()&0222 == 0 {
		log.Logger.Errorw("Dest file does not have writeable permissions", "destName", destName, "err", err)
		return []byte(fmt.Sprintf("#ERROR %s\n", fileName))
	}

	rw := storage.ReadWriter{
		Writer:         destFile,
		Copier:         copier,
		BucketHandle:   bucketHandle,
		BucketName:     config.GetBucket(),
		ObjectName:     object.Name,
		TotalBytes:     object.Size,
		LogDelay:       time.Duration(config.GetLogDelaySec()) * time.Second,
		RateLimitBytes: config.GetRateLimitMb() * 1024 * 1024,
		EncryptionKey:  config.GetEncryptionKey(),
		KMSKey:         config.GetKmsKey(),
	}
	bytesWritten, err := rw.Download(ctx)
	if err != nil {
		log.Logger.Errorw("Error downloading file", "bucket", config.GetBucket(), "destName", destName, "obj", object.Name, "err", err)
		return []byte(fmt.Sprintf("#ERROR %s\n", fileName))
	}
	log.Logger.Infow("File restored", "bucket", config.GetBucket(), "destName", destName, "obj", object.Name, "bytesWritten", bytesWritten)
	externalBackupID = strings.TrimSuffix(filepath.Base(object.Name), ".bak")
	return []byte(fmt.Sprintf("#RESTORED %q %s\n", externalBackupID, fileName))
}
