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

// Package system contains types and functions needed to perform SAP System discovery operations.
package system

import (
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
	"golang.org/x/exp/slices"
	compute "google.golang.org/api/compute/v1"
	file "google.golang.org/api/file/v1"

	"github.com/GoogleCloudPlatform/sapagent/internal/commandlineexecutor"
	"github.com/GoogleCloudPlatform/sapagent/internal/gce/workloadmanager"
	"github.com/GoogleCloudPlatform/sapagent/internal/log"
	"github.com/GoogleCloudPlatform/sapagent/internal/processmetrics/sapdiscovery"
	cpb "github.com/GoogleCloudPlatform/sapagent/protos/configuration"
	ipb "github.com/GoogleCloudPlatform/sapagent/protos/instanceinfo"
	sappb "github.com/GoogleCloudPlatform/sapagent/protos/sapapp"
	spb "github.com/GoogleCloudPlatform/sapagent/protos/system"
)

var (
	ipRegex         = regexp.MustCompile(`[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+`)
	fsMountRegex    = regexp.MustCompile(`([0-9]+\.[0-9]+\.[0-9]+\.[0-9]+):(/[a-zA-Z0-9]+)`)
	sidRegex        = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9]{2})`)
	headerLineRegex = regexp.MustCompile(`[^-]+`)
)

type gceInterface interface {
	GetInstance(project, zone, instance string) (*compute.Instance, error)
	GetInstanceByIP(project, ip string) (*compute.Instance, error)
	GetDisk(project, zone, name string) (*compute.Disk, error)
	GetAddress(project, location, name string) (*compute.Address, error)
	GetAddressByIP(project, region, ip string) (*compute.Address, error)
	GetForwardingRule(project, location, name string) (*compute.ForwardingRule, error)
	GetRegionalBackendService(project, region, name string) (*compute.BackendService, error)
	GetInstanceGroup(project, zone, name string) (*compute.InstanceGroup, error)
	ListInstanceGroupInstances(project, zone, name string) (*compute.InstanceGroupsListInstances, error)
	GetFilestoreByIP(project, location, ip string) (*file.ListInstancesResponse, error)
	GetURIForIP(project, ip string) (string, error)
}

type wlmInterface interface {
	WriteInsight(project, location string, writeInsightRequest *workloadmanager.WriteInsightRequest) error
}

type (
	runCmdAsUser func(user, executable string, args ...string) (string, string, error)
)

// Discovery is a type used to perform SAP System discovery operations.
type Discovery struct {
	gceService        gceInterface
	wlmService        wlmInterface
	exists            commandlineexecutor.CommandExistsRunner
	commandRunner     commandlineexecutor.CommandRunner
	userCommandRunner runCmdAsUser
	hostResolver      func(string) ([]string, error)
}

func extractFromURI(uri, field string) string {
	parts := strings.Split(uri, "/")
	for i, s := range parts {
		if s == field && i+1 < len(parts) {
			return parts[i+1]
		}
	}

	return ""
}

func insightResourceFromSystemResource(r *spb.SapDiscovery_Resource) *workloadmanager.SapDiscoveryResource {

	return &workloadmanager.SapDiscoveryResource{
		RelatedResources: r.RelatedResources,
		ResourceKind:     r.ResourceKind,
		ResourceType:     r.ResourceType.String(),
		ResourceURI:      r.ResourceUri,
		UpdateTime:       r.UpdateTime.AsTime().Format(time.RFC3339),
	}
}

func insightComponentFromSystemComponent(comp *spb.SapDiscovery_Component) *workloadmanager.SapDiscoveryComponent {
	iComp := &workloadmanager.SapDiscoveryComponent{
		HostProject: comp.HostProject,
		Sid:         comp.Sid,
	}

	for _, r := range comp.Resources {
		iComp.Resources = append(iComp.Resources, insightResourceFromSystemResource(r))
	}

	return iComp
}

func insightFromSAPSystem(sys *spb.SapDiscovery) *workloadmanager.Insight {
	iDiscovery := &workloadmanager.SapDiscovery{
		SystemID:   sys.SystemId,
		UpdateTime: sys.UpdateTime.AsTime().Format(time.RFC3339),
	}
	if sys.ApplicationLayer != nil {
		iDiscovery.ApplicationLayer = insightComponentFromSystemComponent(sys.ApplicationLayer)
		iDiscovery.ApplicationLayer.ApplicationType = sys.ApplicationLayer.GetApplicationType()

	}
	if sys.DatabaseLayer != nil {
		iDiscovery.DatabaseLayer = insightComponentFromSystemComponent(sys.DatabaseLayer)
		iDiscovery.DatabaseLayer.DatabaseType = sys.DatabaseLayer.GetDatabaseType()
	}

	return &workloadmanager.Insight{SapDiscovery: iDiscovery}
}

// StartSAPSystemDiscovery Initializes the discovery object and starts the discovery subroutine.
// Returns true if the discovery goroutine is started, and false otherwise.
func StartSAPSystemDiscovery(ctx context.Context, config *cpb.Configuration, gceService gceInterface, wlmService wlmInterface) bool {
	// Start SAP system discovery only if sap_system_discovery is enabled.
	if !config.GetCollectionConfiguration().GetSapSystemDiscovery() {
		log.Logger.Info("Not starting SAP system discovery.")
		return false
	}

	d := Discovery{
		gceService:        gceService,
		wlmService:        wlmService,
		exists:            commandlineexecutor.CommandExists,
		commandRunner:     commandlineexecutor.ExpandAndExecuteCommand,
		userCommandRunner: commandlineexecutor.ExecuteCommandAsUser,
		hostResolver:      net.LookupHost,
	}

	go runDiscovery(config, d)
	return true
}

func runDiscovery(config *cpb.Configuration, d Discovery) {
	cp := config.GetCloudProperties()
	if cp == nil {
		log.Logger.Warn("No Metadata Cloud Properties found, cannot collect resource information from the Compute API")
		return
	}

	for {
		// Discover instance and immediately adjacent resources (disks, addresses, networks)
		res, ci, ir := d.discoverInstance(cp.GetProjectId(), cp.GetZone(), cp.GetInstanceName())

		if ci != nil {
			netRes := d.discoverNetworks(cp.GetProjectId(), ci, ir)
			res = append(res, netRes...)

			disks := d.discoverDisks(cp.GetProjectId(), cp.GetZone(), ci, ir)
			res = append(res, disks...)
		}

		fsRes := d.discoverFilestores(cp.GetProjectId(), ir)
		res = append(res, fsRes...)

		fwrRes, fwr, fr := d.discoverClusterForwardingRule(cp.GetProjectId(), cp.GetZone())
		res = append(res, fwrRes...)

		if fwr != nil {
			lbRes := d.discoverLoadBalancerFromForwardingRule(fwr, fr)
			res = append(res, lbRes...)

			// Only add the unique resources, some may be shared, such as network and subnetwork
			for _, l := range lbRes {
				if idx := slices.IndexFunc(res, func(r *spb.SapDiscovery_Resource) bool { return r.ResourceUri == l.ResourceUri }); idx == -1 {
					res = append(res, l)
				}
			}
		}

		sapApps := sapdiscovery.SAPApplications()

		sapSystems := []*spb.SapDiscovery{}

		for _, app := range sapApps.Instances {
			var system *spb.SapDiscovery
			switch app.Type {
			case sappb.InstanceType_NETWEAVER:
				log.Logger.Info("Running on NW instance")

				var dbComp *spb.SapDiscovery_Component
				dbRes := d.discoverAppToDBConnection(cp, app.Sapsid, ir)
				if len(dbRes) > 0 {
					// NW instance is connected to a database
					dbSid, err := d.discoverDatabaseSID(app.Sapsid)
					if err != nil {
						log.Logger.Warnw("Encountered error discovering database SID", "error", err)
						continue
					}
					dbComp = &spb.SapDiscovery_Component{
						Sid:       dbSid,
						Resources: dbRes,
					}
				}
				// See if a system with the same SID already exists
				for _, sys := range sapSystems {
					if sys.GetApplicationLayer().GetSid() == app.Sapsid || sys.GetDatabaseLayer().GetSid() == dbComp.Sid {
						system = sys
						break
					}
				}
				if system == nil {
					system = &spb.SapDiscovery{}
					sapSystems = append(sapSystems, system)
				}
				system.ApplicationLayer = &spb.SapDiscovery_Component{
					Sid:       app.Sapsid,
					Resources: res,
				}
				if dbComp != nil {
					system.DatabaseLayer = dbComp
				}
				system.UpdateTime = timestamppb.Now()
			case sappb.InstanceType_HANA:
				log.Logger.Info("Running on HANA instance")
				// See if a system with the same SID already exists
				for _, sys := range sapSystems {
					if sys.DatabaseLayer.Sid == app.Sapsid {
						system = sys
						break
					}
				}

				d.discoverDBNodes(app.Sapsid, app.InstanceNumber, cp.ProjectId, cp.Zone)
				if system == nil {
					system = &spb.SapDiscovery{}
					sapSystems = append(sapSystems, system)
				}
				system.DatabaseLayer = &spb.SapDiscovery_Component{
					Sid:       app.Sapsid,
					Resources: res,
				}
				system.UpdateTime = timestamppb.Now()
			}
			log.Logger.Infow("Discovered System", "system", system)
		}

		locationParts := strings.Split(cp.GetZone(), "-")
		continent := locationParts[0]

		log.Logger.Info("Sending systems to WLM API")
		for _, sys := range sapSystems {
			// Send System to DW API
			req := &workloadmanager.WriteInsightRequest{
				Insight: insightFromSAPSystem(sys),
			}
			log.Logger.Infow("Sending write insight request", "request", req)

			d.wlmService.WriteInsight(cp.ProjectId, continent, req)
		}

		log.Logger.Info("Done SAP System Discovery")
		// Perform discovery at most every 4 hours.
		time.Sleep(4 * 60 * 60 * time.Second)
	}
}

func (d *Discovery) discoverInstance(projectID, zone, instanceName string) ([]*spb.SapDiscovery_Resource, *compute.Instance, *spb.SapDiscovery_Resource) {
	var res []*spb.SapDiscovery_Resource
	log.Logger.Debugw("Discovering instance", log.String("instance", instanceName))
	ci, err := d.gceService.GetInstance(projectID, zone, instanceName)
	if err != nil {
		log.Logger.Errorw("Could not get instance info from compute API",
			log.String("project", projectID),
			log.String("zone", zone),
			log.String("instance", instanceName),
			log.Error(err))
		return res, nil, nil
	}

	ir := &spb.SapDiscovery_Resource{
		ResourceType: spb.SapDiscovery_Resource_COMPUTE,
		ResourceKind: "ComputeInstance",
		ResourceUri:  ci.SelfLink,
		UpdateTime:   timestamppb.Now(),
	}
	res = append(res, ir)

	return res, ci, ir
}

func (d *Discovery) discoverDisks(projectID, zone string, ci *compute.Instance, ir *spb.SapDiscovery_Resource) []*spb.SapDiscovery_Resource {
	var disks []*spb.SapDiscovery_Resource
	if ci == nil || ci.Disks == nil || len(ci.Disks) == 0 {
		return disks
	}
	// Get the disks
	for _, disk := range ci.Disks {
		source, diskName := disk.Source, disk.DeviceName

		s := strings.Split(source, "/")
		if len(s) >= 2 {
			diskName = s[len(s)-1]
		}

		cd, err := d.gceService.GetDisk(projectID, zone, diskName)
		if err != nil {
			log.Logger.Warnw("Could not get disk info from compute API",
				log.String("project", projectID),
				log.String("zone", zone),
				log.String("instance", diskName),
				log.Error(err))
			continue
		}

		dr := &spb.SapDiscovery_Resource{
			ResourceType: spb.SapDiscovery_Resource_COMPUTE,
			ResourceKind: "ComputeDisk",
			ResourceUri:  cd.SelfLink,
			UpdateTime:   timestamppb.Now(),
		}
		disks = append(disks, dr)
		ir.RelatedResources = append(ir.RelatedResources, dr.ResourceUri)
	}
	return disks
}

func (d *Discovery) discoverNetworks(projectID string, ci *compute.Instance, ir *spb.SapDiscovery_Resource) []*spb.SapDiscovery_Resource {
	var netRes []*spb.SapDiscovery_Resource
	if ci == nil || ci.NetworkInterfaces == nil || len(ci.NetworkInterfaces) == 0 {
		return netRes
	}
	// Get Network related resources
	for _, net := range ci.NetworkInterfaces {
		sr := &spb.SapDiscovery_Resource{
			ResourceType: spb.SapDiscovery_Resource_COMPUTE,
			ResourceKind: "ComputeSubnetwork",
			ResourceUri:  net.Subnetwork,
			UpdateTime:   timestamppb.Now(),
		}
		netRes = append(netRes, sr)
		ir.RelatedResources = append(ir.RelatedResources, sr.ResourceUri)

		nr := &spb.SapDiscovery_Resource{
			ResourceType: spb.SapDiscovery_Resource_COMPUTE,
			ResourceKind: "ComputeNetwork",
			ResourceUri:  net.Network,
			UpdateTime:   timestamppb.Now(),
		}
		nr.RelatedResources = append(nr.RelatedResources, sr.ResourceUri)
		netRes = append(netRes, nr)
		ir.RelatedResources = append(ir.RelatedResources, nr.ResourceUri)

		// Examine assigned IP addresses
		for _, ac := range net.AccessConfigs {
			ar := &spb.SapDiscovery_Resource{
				ResourceType: spb.SapDiscovery_Resource_COMPUTE,
				ResourceKind: "PublicAddress",
				UpdateTime:   timestamppb.Now(),
				ResourceUri:  ac.NatIP,
			}
			netRes = append(netRes, ar)
			ir.RelatedResources = append(ir.RelatedResources, ar.ResourceUri)
		}

		netRegion := extractFromURI(net.Subnetwork, "regions")
		if netRegion == "" {
			log.Logger.Warnw("Unable to extract region from subnetwork",
				log.String("subnetwork", net.Subnetwork))
			continue
		}

		// Check Network Interface address to see if it exists as a resource
		ip := net.NetworkIP
		addr, err := d.gceService.GetAddressByIP(projectID, netRegion, ip)
		if err != nil {
			log.Logger.Warnw("Error locating Address by IP",
				log.String("project", projectID),
				log.String("region", netRegion),
				log.String("ip", ip),
				log.Error(err))
			continue
		}
		ar := &spb.SapDiscovery_Resource{
			ResourceType: spb.SapDiscovery_Resource_COMPUTE,
			ResourceKind: "ComputeAddress",
			ResourceUri:  addr.SelfLink,
			UpdateTime:   timestamppb.Now(),
		}
		sr.RelatedResources = append(sr.RelatedResources, ar.ResourceUri)
		netRes = append(netRes, ar)
		ir.RelatedResources = append(ir.RelatedResources, ar.ResourceUri)
	}
	return netRes
}

func (d *Discovery) discoverClusterForwardingRule(projectID, zone string) ([]*spb.SapDiscovery_Resource, *compute.ForwardingRule, *spb.SapDiscovery_Resource) {
	var res []*spb.SapDiscovery_Resource
	lbAddress, err := d.discoverCluster()
	if err != nil || lbAddress == "" {
		log.Logger.Warnw("Encountered error discovering cluster address", log.Error(err))
		return res, nil, nil
	}

	// With address in hand we can find what it is assigned to
	region := strings.Join(strings.Split(zone, "-")[0:2], "-")
	// Check Network Interface address to see if it exists as a resource
	addr, err := d.gceService.GetAddressByIP(projectID, region, lbAddress)
	if err != nil {
		log.Logger.Warnw("Error locating Address by IP",
			log.String("project", projectID),
			log.String("region", region),
			log.String("ip", lbAddress),
			log.Error(err))
		return res, nil, nil
	}

	ar := &spb.SapDiscovery_Resource{
		ResourceType: spb.SapDiscovery_Resource_COMPUTE,
		ResourceKind: "ComputeAddress",
		ResourceUri:  addr.SelfLink,
		UpdateTime:   timestamppb.Now(),
	}
	res = append(res, ar)

	if len(addr.Users) == 0 {
		log.Logger.Warn("Cluster address not in use by anything")
		return res, nil, nil
	}

	// Examine the user of the address, it should be a forwarding rule.
	user := addr.Users[0]
	name := extractFromURI(user, "forwardingRules")
	if name == "" {
		log.Logger.Infow("Cluster address not in use by forwarding rule", log.String("user", user))
		return res, nil, nil
	}
	fwr, err := d.gceService.GetForwardingRule(projectID, region, name)
	if err != nil {
		log.Logger.Warnw("Error retrieving forwarding rule", log.Error(err))
		return res, nil, nil
	}

	fr := &spb.SapDiscovery_Resource{
		ResourceType:     spb.SapDiscovery_Resource_COMPUTE,
		ResourceKind:     "ComputeForwardingRule",
		ResourceUri:      fwr.SelfLink,
		RelatedResources: []string{ar.ResourceUri},
		UpdateTime:       timestamppb.Now(),
	}
	ar.RelatedResources = append(ar.RelatedResources, fr.ResourceUri)
	res = append(res, fr)

	return res, fwr, fr
}

func (d *Discovery) discoverLoadBalancerFromForwardingRule(fwr *compute.ForwardingRule, fr *spb.SapDiscovery_Resource) []*spb.SapDiscovery_Resource {
	log.Logger.Debug("Discovering load balancer")
	var res []*spb.SapDiscovery_Resource
	projectID := extractFromURI(fwr.SelfLink, "projects")

	// Examine fwr backend service, this should be the load balancer
	b := fwr.BackendService
	bEName := extractFromURI(b, "backendServices")
	if bEName == "" {
		log.Logger.Infow("Forwarding rule does not have a backend service",
			log.String("bakendService", b))
		return res
	}

	bERegion := extractFromURI(b, "regions")
	if bERegion == "" {
		log.Logger.Infow("Unable to extract region from backend service", log.String("backendService", b))
		return res
	}

	bs, err := d.gceService.GetRegionalBackendService(projectID, bERegion, bEName)
	if err != nil {
		log.Logger.Warnw("Error retrieving backend service", log.Error(err))
		return res
	}

	bsr := &spb.SapDiscovery_Resource{
		ResourceType:     spb.SapDiscovery_Resource_COMPUTE,
		ResourceKind:     "ComputeBackendService",
		ResourceUri:      bs.SelfLink,
		UpdateTime:       timestamppb.Now(),
		RelatedResources: []string{fr.ResourceUri},
	}
	fr.RelatedResources = append(fr.RelatedResources, bsr.ResourceUri)
	res = append(res, bsr)

	igRes := d.discoverInstanceGroups(bs, bsr)
	res = append(res, igRes...)
	return res
}

func (d *Discovery) discoverInstanceGroups(bs *compute.BackendService, parent *spb.SapDiscovery_Resource) []*spb.SapDiscovery_Resource {
	projectID := extractFromURI(bs.SelfLink, "projects")
	var res []*spb.SapDiscovery_Resource
	var groups []string
	for _, be := range bs.Backends {
		if be.Group != "" {
			groups = append(groups, be.Group)
		}
	}

	for _, g := range groups {
		gName := extractFromURI(g, "instanceGroups")
		if gName == "" {
			log.Logger.Info("Backend group is not an instance group")
			continue
		}
		gZone := extractFromURI(g, "zones")
		if gZone == "" {
			log.Logger.Info("Unable to extract zone from group name")
			continue
		}

		ig, err := d.gceService.GetInstanceGroup(projectID, gZone, gName)
		if err != nil {
			log.Logger.Warnw("Error retrieving instance group", log.Error(err))
			continue
		}
		igr := &spb.SapDiscovery_Resource{
			ResourceType: spb.SapDiscovery_Resource_COMPUTE,
			ResourceKind: "ComputeInstanceGroup",
			ResourceUri:  ig.SelfLink,
			UpdateTime:   timestamppb.Now(),
		}
		parent.RelatedResources = append(parent.RelatedResources, igr.ResourceUri)
		res = append(res, igr)

		iRes := d.discoverInstanceGroupInstances(projectID, gZone, gName, igr)
		res = append(res, iRes...)
	}

	return res
}

func (d *Discovery) discoverInstanceGroupInstances(projectID, zone, name string, parent *spb.SapDiscovery_Resource) []*spb.SapDiscovery_Resource {
	var res []*spb.SapDiscovery_Resource
	list, err := d.gceService.ListInstanceGroupInstances(projectID, zone, name)
	if err != nil {
		log.Logger.Warnw("Error retrieving instance group instances", log.Error(err))
		return res
	}

	var instances []string
	for _, i := range list.Items {
		parent.RelatedResources = append(parent.RelatedResources, i.Instance)
		iName := extractFromURI(i.Instance, "instances")
		if iName == "" {
			log.Logger.Warnw("Unable to extract instance name from instance group items",
				log.String("item", i.Instance))
			continue
		}
		instances = append(instances, i.Instance)
	}

	for _, i := range instances {
		iName := extractFromURI(i, "instances")
		if iName == "" {
			log.Logger.Warnw("Unable to extract instance name from instance group items", log.String("item", i))
			continue
		}
		iProject := extractFromURI(i, "projects")
		if iProject == "" {
			log.Logger.Warnw("Unable to extract project from instance group items", log.String("item", i))
			continue
		}
		iZone := extractFromURI(i, "zones")
		if iZone == "" {
			log.Logger.Warnw("Unable to extract zone from instance group items", log.String("item", i))
			continue
		}
		instanceRes, ci, ir := d.discoverInstance(iProject, iZone, iName)
		res = append(res, instanceRes...)

		netRes := d.discoverNetworks(iProject, ci, ir)
		res = append(res, netRes...)

		disks := d.discoverDisks(iProject, iZone, ci, ir)
		res = append(res, disks...)

	}
	return res
}

func (d *Discovery) discoverCluster() (string, error) {
	log.Logger.Info("Discovering cluster")
	if d.exists("crm") {
		stdOut, _, err := d.commandRunner("crm", "config show")
		if err != nil {
			return "", err
		}

		var addrPrimitiveFound bool
		for _, l := range strings.Split(stdOut, "\n") {
			if strings.Contains(l, "rsc_vip_int-primary IPaddr2") {
				addrPrimitiveFound = true
			}
			if addrPrimitiveFound && strings.Contains(l, "params ip") {
				address := ipRegex.FindString(l)
				if address == "" {
					return "", errors.New("Unable to locate IP address in crm output: " + stdOut)
				}
				return address, nil
			}
		}
		return "", errors.New("No address found in pcs cluster config output")
	}
	if d.exists("pcs") {
		stdOut, _, err := d.commandRunner("pcs", "config show")
		if err != nil {
			return "", err
		}

		var addrPrimitiveFound bool
		for _, l := range strings.Split(stdOut, "\n") {
			if addrPrimitiveFound && strings.Contains(l, "ip") {
				address := ipRegex.FindString(l)
				if address == "" {
					return "", errors.New("Unable to locate IP address in crm output: " + stdOut)
				}
				return address, nil
			}
			if strings.Contains(l, "rsc_vip_") {
				addrPrimitiveFound = true
			}
		}
		return "", errors.New("No address found in pcs cluster config output")
	}
	return "", errors.New("No cluster command found")
}

func (d *Discovery) discoverFilestores(projectID string, parent *spb.SapDiscovery_Resource) []*spb.SapDiscovery_Resource {
	log.Logger.Info("Discovering mounted file stores")
	var res []*spb.SapDiscovery_Resource
	if !d.exists("df") {
		log.Logger.Warn("Cannot access command df to discover mounted file stores")
		return res
	}

	stdOut, _, err := d.commandRunner("df", "-h")
	if err != nil {
		log.Logger.Warnw("Error retrieving mounts", "error", err)
		return res
	}
	for _, l := range strings.Split(stdOut, "\n") {
		matches := fsMountRegex.FindStringSubmatch(l)
		if len(matches) < 2 {
			continue
		}
		// The first match is the fully matched string, we only need the first submatch, the IP address.
		address := matches[1]
		fs, err := d.gceService.GetFilestoreByIP(projectID, "-", address)
		if err != nil {
			log.Logger.Errorw("Error retrieving filestore by IP", "error", err)
			continue
		} else if len(fs.Instances) == 0 {
			log.Logger.Warnw("No filestore found with IP", "address", address)
			continue
		}
		for _, i := range fs.Instances {
			fsr := &spb.SapDiscovery_Resource{
				ResourceType:     spb.SapDiscovery_Resource_STORAGE,
				ResourceKind:     "ComputeFilestore",
				ResourceUri:      i.Name,
				RelatedResources: []string{parent.ResourceUri},
				UpdateTime:       timestamppb.Now(),
			}
			parent.RelatedResources = append(parent.RelatedResources, fsr.ResourceUri)
			res = append(res, fsr)
		}
	}

	return res
}

func (d *Discovery) discoverAppToDBConnection(cp *ipb.CloudProperties, sid string, parent *spb.SapDiscovery_Resource) []*spb.SapDiscovery_Resource {
	var res []*spb.SapDiscovery_Resource

	sidLower := strings.ToLower(sid)
	sidUpper := strings.ToUpper(sid)
	sidPath := fmt.Sprintf("/usr/sap/%s/hdbclient/hdbuserstore", sidUpper)
	sidAdm := fmt.Sprintf("%sadm", sidLower)
	stdOut, stdErr, err := d.userCommandRunner(sidAdm, sidPath, "list", "DEFAULT")
	if err != nil {
		log.Logger.Warnw("Error retrieving hdbuserstore info", "sid", sid, "error", err, "stdout", stdOut, "stderr", stdErr)
		return res
	}

	outLines := strings.Split(stdOut, "\n")
	log.Logger.Infof("outLines: %v", outLines)
	var dbHosts []string
	for _, l := range outLines {
		log.Logger.Infow("Examining line", "line", l)
		t := strings.TrimSpace(l)
		if strings.Index(t, "ENV") < 0 {
			log.Logger.Info("No ENV")
			continue
		}

		log.Logger.Infof("Env line: %s", t)
		// Trim up to the first colon
		_, hosts, _ := strings.Cut(t, ":")
		p := strings.Split(hosts, ";")
		// Each semicolon part contains the pattern <host>:<port>
		// The first part will contain "ENV : <host>:port"
		for _, h := range p {
			log.Logger.Infof("Semicolon part: %s", h)
			c := strings.Split(h, ":")
			if len(c) < 2 {
				continue
			}
			dbHosts = append(dbHosts, strings.TrimSpace(c[0]))
		}
	}
	if len(dbHosts) == 0 {
		log.Logger.Warnw("Unable to find DB hostname and port in hdbuserstore output", "sid", sid)
		return res
	}

	for _, dbHostname := range dbHosts {
		log.Logger.Infow("Found host", "sid", sid, "hostname", fmt.Sprintf("%q", dbHostname))

		addrs, err := d.hostResolver(dbHostname)
		if err != nil {
			log.Logger.Warn("Error retrieving address, or no address found for host", log.String("sid", sid), log.String("hostname", dbHostname), log.Error(err))
			return res
		}

		for _, ip := range addrs {
			log.Logger.Info("Examining address", log.String("sid", sid), log.String("ip", ip))
			addressURI, err := d.gceService.GetURIForIP(cp.GetProjectId(), ip)
			if err != nil {
				log.Logger.Warnw("Error finding URI for IP", "IP", ip, "error", err)
				continue
			}

			switch {
			case extractFromURI(addressURI, "addresses") != "":
				aRes := d.discoverAddressFromURI(addressURI)
				res = append(res, aRes...)
			case extractFromURI(addressURI, "instances") != "":
				// IP is assigned to an instance
				iRes := d.discoverInstanceFromURI(addressURI)
				res = append(res, iRes...)
			default:
				log.Logger.Infow("Unrecognized URI type for IP", "IP", ip, "URI", addressURI)
				continue
			}
		}
	}
	return res
}

func (d *Discovery) discoverInstanceFromURI(instanceURI string) []*spb.SapDiscovery_Resource {
	var res []*spb.SapDiscovery_Resource
	iName := extractFromURI(instanceURI, "instances")
	iZone := extractFromURI(instanceURI, "zones")
	iProject := extractFromURI(instanceURI, "projects")
	if iName == "" || iProject == "" || iZone == "" {
		log.Logger.Warnw("Unable to extract instance information from user URI", "instanceURI", instanceURI)
		return res
	}

	iRes, ci, ir := d.discoverInstance(iProject, iZone, iName)
	res = append(res, iRes...)
	if ir == nil {
		return res
	}

	netRes := d.discoverNetworks(iProject, ci, ir)
	res = append(res, netRes...)

	disks := d.discoverDisks(iProject, iZone, ci, ir)
	res = append(res, disks...)
	return res
}

func (d *Discovery) discoverForwardingRuleFromURI(fwrURI string) []*spb.SapDiscovery_Resource {
	var res []*spb.SapDiscovery_Resource
	fwrName := extractFromURI(fwrURI, "forwardingRules")
	fwrProject := extractFromURI(fwrURI, "projects")
	fwrLocation := extractFromURI(fwrURI, "zones")
	if fwrLocation == "" {
		fwrLocation = extractFromURI(fwrURI, "regions")
	}
	if fwrLocation == "" && !strings.Contains(fwrURI, "/global/") {
		log.Logger.Warn("Unknown location type for forwarding rule", "fwrURI", fwrURI)
		return res
	}

	fwr, err := d.gceService.GetForwardingRule(fwrProject, fwrLocation, fwrName)
	if err != nil {
		log.Logger.Warn("Error retrieving forwarding rule", log.String("fwrName", fwrName), log.Error(err))
		return res
	}

	fr := &spb.SapDiscovery_Resource{
		ResourceType: spb.SapDiscovery_Resource_COMPUTE,
		ResourceKind: "ComputeForwardingRule",
		ResourceUri:  fwr.SelfLink,
		UpdateTime:   timestamppb.Now(),
	}
	res = append(res, fr)

	lbRes := d.discoverLoadBalancerFromForwardingRule(fwr, fr)
	res = append(res, lbRes...)

	return res
}

func (d *Discovery) discoverAddressFromURI(addressURI string) []*spb.SapDiscovery_Resource {
	var res []*spb.SapDiscovery_Resource
	addrProject := extractFromURI(addressURI, "projects")
	addrLocation := extractFromURI(addressURI, "zones")
	addrName := extractFromURI(addressURI, "addresses")
	if addrLocation == "" {
		addrLocation = extractFromURI(addressURI, "regions")
	}
	if addrLocation == "" && !strings.Contains(addressURI, "/global/") {
		log.Logger.Warnw("Unknown location type for address", "addressURI", addressURI)
		return res
	}
	// IP is assigned to an address
	log.Logger.Info("Address found")
	ar := &spb.SapDiscovery_Resource{
		ResourceType: spb.SapDiscovery_Resource_COMPUTE,
		ResourceKind: "ComputeAddress",
		ResourceUri:  addressURI,
		UpdateTime:   timestamppb.Now(),
	}
	res = append(res, ar)
	// parent.RelatedResources = append(parent.RelatedResources, ar.ResourceUri)

	addr, err := d.gceService.GetAddress(addrProject, addrLocation, addrName)
	if err != nil {
		log.Logger.Warnw("Error retrieving address", "error", err)
		return res
	}

	res = append(res, d.discoverAddressUsers(addr)...)

	return res
}

func (d *Discovery) discoverAddressUsers(addr *compute.Address) []*spb.SapDiscovery_Resource {
	var res []*spb.SapDiscovery_Resource
	// IP is associated with an address
	// Is that address assigned to an instance or a load balancer
	if len(addr.Users) == 0 {
		// No users
		log.Logger.Warn("ComputeAddress has no users")
		return res
	}

	for _, user := range addr.Users {
		switch {
		case extractFromURI(user, "instances") != "":
			// Address' user is a ComputeInstance
			iRes := d.discoverInstanceFromURI(user)
			res = append(res, iRes...)
		case extractFromURI(user, "forwardingRules") != "":
			log.Logger.Info("User is forwarding rule")
			fRes := d.discoverForwardingRuleFromURI(user)
			res = append(res, fRes...)
		default:
			log.Logger.Warnw("Unknown address user type for address", "addrUser", user)
		}
	}

	return res
}

func (d *Discovery) discoverDatabaseSID(appSID string) (string, error) {
	sidLower := strings.ToLower(appSID)
	sidUpper := strings.ToUpper(appSID)
	sidPath := fmt.Sprintf("/usr/sap/%s/hdbclient/hdbuserstore", sidUpper)
	sidAdm := fmt.Sprintf("%sadm", sidLower)
	stdOut, stdErr, err := d.userCommandRunner(sidAdm, sidPath, "list")
	if err != nil {
		log.Logger.Warnw("Error retrieving hdbuserstore info", "sid", appSID, "error", err, "stdOut", stdOut, "stdErr", stdErr)
		return "", err
	}

	re, err := regexp.Compile(`DATABASE\s*:\s*([a-zA-Z][a-zA-Z0-9]{2})`)
	if err != nil {
		log.Logger.Warnw("Error compiling regex", "error", err)
		return "", err
	}
	sid := re.FindStringSubmatch(stdOut)
	if len(sid) > 1 {
		return sid[1], nil
	}

	// No DB SID in userstore, check profiles
	profilePath := fmt.Sprintf("/usr/sap/%s/SYS/profile/*", sidUpper)
	stdOut, stdErr, err = d.commandRunner("sh", `-c 'grep "dbid\|dbms/name" `+profilePath+`'`)
	if err != nil {
		log.Logger.Warnw("Error retrieving sap profile info", "sid", appSID, "error", err, "stdOut", stdOut, "stdErr", stdErr)
		return "", err
	}

	re, err = regexp.Compile(`(dbid|dbms\/name)\s*=\s*([a-zA-Z][a-zA-Z0-9]{2})`)
	if err != nil {
		log.Logger.Warnw("Error compiling regex", "error", err)
		return "", err
	}
	sid = re.FindStringSubmatch(stdOut)
	if len(sid) > 2 {
		log.Logger.Infow("Found DB SID", "sid", sid[2])
		return sid[2], nil
	}

	return "", errors.New("No database SID found")
}

func (d *Discovery) discoverDBNodes(sid, instanceNumber, project, zone string) []*spb.SapDiscovery_Resource {
	var res []*spb.SapDiscovery_Resource
	if sid == "" || instanceNumber == "" || project == "" || zone == "" {
		log.Logger.Warn("To discover additional HANA nodes SID, instance number, project, and zone must be provided")
		return res
	}
	sidLower := strings.ToLower(sid)
	sidUpper := strings.ToUpper(sid)
	sidAdm := fmt.Sprintf("%sadm", sidLower)
	scriptPath := fmt.Sprintf("/usr/sap/%s/HDB%s/exe/python_support/landscapeHostConfiguration.py", sidUpper, instanceNumber)
	command := fmt.Sprintf("-i -u %s python %s", sidAdm, scriptPath)
	stdOut, stdErr, err := d.commandRunner("sudo", command)
	// Only 0 and 1 are expected error return codes from this script
	if err != nil && commandlineexecutor.ExitCode(err) < 2 {
		log.Logger.Warnw("Error running landscapeHostConfiguration.py", "sid", sid, "error", err, "stdOut", stdOut, "stdErr", stdErr)
		return res
	}

	// Example output:
	// | Host        | Host   | Host   | Failover | Remove | Storage   | Storage   | Failover | Failover | NameServer | NameServer | IndexServer | IndexServer | Host    | Host    | Worker  | Worker  |
	// |             | Active | Status | Status   | Status | Config    | Actual    | Config   | Actual   | Config     | Actual     | Config      | Actual      | Config  | Actual  | Config  | Actual  |
	// |             |        |        |          |        | Partition | Partition | Group    | Group    | Role       | Role       | Role        | Role        | Roles   | Roles   | Groups  | Groups  |
	// | ----------- | ------ | ------ | -------- | ------ | --------- | --------- | -------- | -------- | ---------- | ---------- | ----------- | ----------- | ------- | ------- | ------- | ------- |
	// | dru-s4dan   | yes    | ok     |          |        |         1 |         1 | default  | default  | master 1   | master     | worker      | master      | worker  | worker  | default | default |
	// | dru-s4danw1 | yes    | ok     |          |        |         2 |         2 | default  | default  | master 2   | slave      | worker      | slave       | worker  | worker  | default | default |
	// | dru-s4danw2 | yes    | ok     |          |        |         3 |         3 | default  | default  | slave      | slave      | worker      | slave       | worker  | worker  | default | default |
	// | dru-s4danw3 | yes    | ignore |          |        |         0 |         0 | default  | default  | master 3   | slave      | standby     | standby     | standby | standby | default | -       |
	var hosts []string
	lines := strings.Split(stdOut, "\n")
	pastHeaders := false
	for _, line := range lines {
		log.Logger.Info(line)
		cols := strings.Split(line, "|")
		if len(cols) < 2 {
			log.Logger.Info("Line has too few columns")
			continue
		}
		trimmed := strings.TrimSpace(cols[1])
		if trimmed == "" {
			continue
		}
		if !pastHeaders {
			pastHeaders = !headerLineRegex.MatchString(trimmed)
			continue
		}

		hosts = append(hosts, trimmed)
	}
	log.Logger.Infow("Discovered other hosts", "sid", sid, "hosts", hosts)

	for _, host := range hosts {
		iRes, _, _ := d.discoverInstance(project, zone, host)
		res = append(res, iRes...)
	}

	return res

}
