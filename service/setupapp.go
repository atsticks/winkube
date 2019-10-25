// Copyright 2019 Anatole Tresch
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/**
 * Setup Application registered under /setup on startup if no valid config is present.
 */
package service

import (
	"fmt"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"github.com/winkube/util"
	"github.com/winkube/webapp"
	"golang.org/x/text/language"
	"net"
	"net/http"
	"strconv"
	"strings"
)

func SetupWebApplication(router *mux.Router) *webapp.WebApplication {
	log.Info("Initializing setup...")
	setupWebapp := webapp.CreateWebApp("WinKube-Setup", "/setup", language.English)
	// Pages
	setupWebapp.AddPage(&webapp.Page{
		Name:     "index",
		Template: "templates/setup/index.html",
	}).AddPage(&webapp.Page{
		Name:     "step1",
		Template: "templates/setup/step1.html",
	}).AddPage(&webapp.Page{
		Name:     "step2",
		Template: "templates/setup/step2.html",
	}).AddPage(&webapp.Page{
		Name:     "step3",
		Template: "templates/setup/step3.html",
	})
	// Actions
	setupWebapp.SetAction("/", IndexAction)
	setupWebapp.SetAction("/step1", Step1Action)
	setupWebapp.SetAction("/step2", Step2Action)
	setupWebapp.SetAction("/step3", Step3Action)
	setupWebapp.SetAction("/install", InstallConfigAction)
	return setupWebapp
}

type ConfigBean struct {
	Values            *AppConfiguration
	UseBridgedNetwork bool
	UseNATNetwork     bool
}

func readConfig(context *webapp.RequestContext) ConfigBean {
	context.Request.ParseMultipartForm(32000)
	bean := ConfigBean{
		Values: Container().Config,
	}
	if context.GetParameter("UseExistingCluster") != "" {
		Container().Config.UseExistingCluster =
			util.ParseBool(context.GetParameter("UseExistingCluster"))
	}
	if context.GetParameter("Net-MulticastEnabled") != "" {
		Container().Config.NetMulticastEnabled =
			util.ParseBool(context.GetParameter("Net-MulticastEnabled"))
	}
	if context.GetParameter("Net-UPnPPort") != "" {
		upnpPort, err := strconv.Atoi(context.GetParameter("Net-UPnPPort"))
		if err != nil {
			upnpPort = 1900
		}
		Container().Config.NetUPnPPort = upnpPort
	}
	if context.GetParameter("Net-LookupMasters") != "" {
		Container().Config.NetLookupMasters =
			context.GetParameter("Net-LookupMasters")
	}
	if context.GetParameter("Net-Interface") != "" {
		Container().Config.NetHostInterface =
			context.GetParameter("Net-Interface")
	}
	if context.GetParameter("Cluster-ID") != "" {
		Container().Config.ClusterID =
			context.GetParameter("Cluster-ID")
	}
	if context.GetParameter("Cluster-Credentials") != "" {
		Container().Config.ClusterCredentials =
			context.GetParameter("Cluster-Credentials")
	}
	if context.GetParameter("Cluster-PodCIDR") != "" {
		Container().Config.ClusterPodCIDR =
			context.GetParameter("Cluster-PodCIDR")
	}
	if context.GetParameter("Cluster-NetType") != "" {
		Container().Config.ClusterVMNet =
			context.GetParameter("Cluster-NetType")
	}
	if context.GetParameter("Cluster-MasterApiPort") != "" {
		port, err := strconv.Atoi(context.GetParameter("Cluster-MasterApiPort"))
		if err == nil {
			Container().Config.ClusterMasterApiPort = port
		}
	}
	if context.GetParameter("Cluster-ServiceDomain") != "" {
		Container().Config.ClusterServiceDomain =
			context.GetParameter("Cluster-ServiceDomain")
	}
	if context.GetParameter("Node-NetBridgeCIDR") != "" {
		Container().Config.NodeNetBridgeCIDR =
			context.GetParameter("Node-NetBridgeCIDR")
	}
	if context.GetParameter("Node-NetNodeIP") != "" {
		Container().Config.NodeNetNodeIP =
			context.GetParameter("Node-NetNodeIP")
	}
	if context.GetParameter("Node-Type") != "" {
		Container().Config.NodeType =
			nodeTypeFromString(context.GetParameter("Node-Type"))
	}
	if context.GetParameter("Node-Name") != "" {
		Container().Config.NodeName =
			context.GetParameter("Node-Name")
	}
	if context.GetParameter("Node-Box") != "" {
		Container().Config.NodeBox =
			context.GetParameter("Node-Box")
	}
	if context.GetParameter("Node-BoxVersion") != "" {
		Container().Config.NodeBoxVersion =
			context.GetParameter("Node-BoxVersion")
	}
	if context.GetParameter("Node-Memory") != "" {
		val, err := strconv.Atoi(context.GetParameter("Node-Memory"))
		if err == nil {
			Container().Config.NodeMemory = val
		}
	}
	if context.GetParameter("Node-CPU") != "" {
		val, err := strconv.Atoi(context.GetParameter("Node-CPU"))
		if err == nil {
			Container().Config.NodeCPU = val
		}
	}

	bean.UseBridgedNetwork = Container().Config.ClusterVMNet == "Bridge"
	bean.UseNATNetwork = Container().Config.ClusterVMNet == "NAT"
	if bean.UndefinedNode() {
		if bean.Values.UseExistingCluster {
			bean.Values.NodeType = WorkerNode
		} else {
			bean.Values.NodeType = MasterNode
		}
	}
	return bean
}
func nodeTypeFromString(nodeType string) NodeType {
	switch nodeType {
	case "WorkerNode":
		return WorkerNode
	case "MasterNode":
		return MasterNode
	case "MonitorNode":
		return MonitorNode
	default:
		return UndefinedNodeType
	}
}

func (this ConfigBean) WorkerNode() bool {
	return Container().Config.NodeType == WorkerNode
}
func (this ConfigBean) MasterNode() bool {
	return Container().Config.NodeType == MasterNode
}
func (this ConfigBean) MonitorNode() bool {
	return Container().Config.NodeType == MonitorNode
}
func (this ConfigBean) UndefinedNode() bool {
	return Container().Config.NodeType == UndefinedNodeType
}

// Web action starting the setup process

func IndexAction(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	data := make(map[string]interface{})
	data["Config"] = Container().Config
	return &webapp.ActionResponse{
		NextPage: "index",
		Model:    data,
	}
}
func Step1Action(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	// Collect messages
	data := make(map[string]interface{})
	bean := readConfig(context)
	data["Config"] = bean
	return &webapp.ActionResponse{
		NextPage: "step1",
		Model:    data,
	}
}
func Step2Action(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	bean := readConfig(context)
	data := make(map[string]interface{})
	data["Config"] = bean
	data["Clusters"] = clusterOptions(Container().ClusterManager)
	data["Interfaces"] = interfaceOptions(Container().Config.NetHostInterface)
	// Check if node type is set...
	return &webapp.ActionResponse{
		NextPage: "step2",
		Model:    data,
	}
}

func clusterOptions(clusterManager *ClusterManager) webapp.Options {
	clusterOptions := webapp.Options{}
	clusterIds := (*clusterManager).GetClusterIDs()
	// TODO remove this block
	clusterIds = append(clusterIds, "ClusterID-1")
	clusterIds = append(clusterIds, "ClusterID-2")
	clusterIds = append(clusterIds, "ClusterID-3")
	// TODO remove this block
	for _, id := range clusterIds {
		option := webapp.Option{
			Name:     id,
			Value:    id,
			Selected: Container().Config.ClusterID == id,
		}
		clusterOptions.Entries = append(clusterOptions.Entries, option)
	}
	return clusterOptions
}

func interfaceOptions(selected string) webapp.Options {
	ifCurrent := Container().Config.NetHostInterface
	if ifCurrent == "" {
		ifCurrent = "Ethernet"
	}
	interfacesOptions := webapp.Options{}
	ifaces, _ := net.Interfaces() //here your interface
	for _, iface := range ifaces {
		option := webapp.Option{
			Name:     iface.Name + "( " + iface.HardwareAddr.String() + ")" + getDisplayAddr(iface.Addrs()),
			Value:    iface.Name,
			Selected: ifCurrent == iface.Name,
		}
		interfacesOptions.Entries = append(interfacesOptions.Entries, option)
	}
	return interfacesOptions
}

func getDisplayAddr(addresses []net.Addr, err error) string {
	if err != nil {
		return "-"
	}
	b := strings.Builder{}
	for _, address := range addresses {
		switch v := address.(type) {
		case *net.IPNet:
			if !v.IP.IsLoopback() {
				if v.IP.To4() != nil { //Verify if IP is IPV4
					b.WriteString(v.IP.String())
					b.WriteString(",")
				}
			}
		}
	}
	return b.String()
}

// Web action continuing the setup process to step two
func Step3Action(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	// Collect messages
	data := make(map[string]interface{})
	bean := readConfig(context)
	data["Config"] = bean
	return &webapp.ActionResponse{
		NextPage: "step3",
		Model:    data,
	}
}

// Web action starting the node after the configuration has been completed
func InstallConfigAction(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	nodeManager := *Container().NodeManager
	_, err := nodeManager.ValidateConfig()
	if err != nil {
		fmt.Println("Validation failed: " + err.Error())
	} else {
		fmt.Println("Validation successful...")
	}
	action := nodeManager.Initialize(true)
	if action.Error == nil {
		nodeManager.StartNode()
	}
	return &webapp.ActionResponse{
		NextPage: "_redirect",
		Model:    "/actions",
	}
}
