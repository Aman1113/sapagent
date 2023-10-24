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

// Package hostdiscovery contains functions for performing SAP System discovery operations available only on the current host.
package hostdiscovery

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/GoogleCloudPlatform/sapagent/shared/log"

	"github.com/GoogleCloudPlatform/sapagent/shared/commandlineexecutor"
)

var (
	fsMountRegex = regexp.MustCompile(`([0-9]+\.[0-9]+\.[0-9]+\.[0-9]+):(/[a-zA-Z0-9]+)`)
	ipRegex      = regexp.MustCompile(`[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+`)
)

// HostDiscovery is for discovering details that can only be performed on the host running the agent.
type HostDiscovery struct {
	exists  commandlineexecutor.Exists
	execute commandlineexecutor.Execute
}

// DiscoverCurrentHost invokes the necessary commands to discover the resources visible only
// on the current host.
func (d *HostDiscovery) DiscoverCurrentHost(ctx context.Context) []string {
	fs := d.discoverFilestores(ctx)

	addr, err := d.discoverClusterAddress(ctx)
	if err != nil {
		log.CtxLogger(ctx).Warnw("Error received when discovering cluster address", "error", err)
		return fs
	}

	return append(fs, addr)
}

func (d *HostDiscovery) discoverClusterAddress(ctx context.Context) (string, error) {
	log.CtxLogger(ctx).Info("Discovering cluster")
	if d.exists("crm") {
		return d.discoverClusterCRM(ctx)
	}
	if d.exists("pcs") {
		return d.discoverClusterPCS(ctx)
	}
	return "", errors.New("no cluster command found")
}

func (d *HostDiscovery) discoverClusterCRM(ctx context.Context) (string, error) {
	result := d.execute(ctx, commandlineexecutor.Params{
		Executable:  "crm",
		ArgsToSplit: "config show",
	})
	if result.Error != nil {
		return "", result.Error
	}

	var addrPrimitiveFound bool
	for _, l := range strings.Split(result.StdOut, "\n") {
		if strings.Contains(l, "rsc_vip_int-primary IPaddr2") {
			addrPrimitiveFound = true
		}
		if addrPrimitiveFound && strings.Contains(l, "params ip") {
			address := ipRegex.FindString(l)
			if address == "" {
				return "", errors.New("unable to locate IP address in crm output: " + result.StdOut)
			}
			return address, nil
		}
	}
	return "", errors.New("no address found in crm cluster config output")
}

func (d *HostDiscovery) discoverClusterPCS(ctx context.Context) (string, error) {
	result := d.execute(ctx, commandlineexecutor.Params{
		Executable:  "pcs",
		ArgsToSplit: "config show",
	})
	if result.Error != nil {
		return "", result.Error
	}

	var addrPrimitiveFound bool
	for _, l := range strings.Split(result.StdOut, "\n") {
		if addrPrimitiveFound && strings.Contains(l, "ip") {
			address := ipRegex.FindString(l)
			if address == "" {
				return "", errors.New("unable to locate IP address in pcs output: " + result.StdOut)
			}
			return address, nil
		}
		if strings.Contains(l, "rsc_vip_") {
			addrPrimitiveFound = true
		}
	}
	return "", errors.New("no address found in pcs cluster config output")
}

func (d *HostDiscovery) discoverFilestores(ctx context.Context) []string {
	log.CtxLogger(ctx).Info("Discovering mounted file stores")
	if !d.exists("df") {
		log.CtxLogger(ctx).Warn("Cannot access command df to discover mounted file stores")
		return nil
	}

	result := d.execute(ctx, commandlineexecutor.Params{
		Executable: "df",
		Args:       []string{"-h"},
	})
	if result.Error != nil {
		log.CtxLogger(ctx).Warnw("Error retrieving mounts", "error", result.Error)
		return nil
	}
	fs := []string{}
	for _, l := range strings.Split(result.StdOut, "\n") {
		log.CtxLogger(ctx).Infof("Processing mount: %s", l)
		matches := fsMountRegex.FindStringSubmatch(l)
		if len(matches) < 2 {
			log.CtxLogger(ctx).Info("Insufficient matches for line")
			continue
		}
		// The first match is the fully matched string, we only need the first submatch, the IP address.
		address := matches[1]
		log.CtxLogger(ctx).Infof("Found address: %s", address)
		fs = append(fs, address)
	}

	return fs
}
