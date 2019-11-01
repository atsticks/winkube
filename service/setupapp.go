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
	"github.com/winkube/util"
	"github.com/winkube/webapp"
	"golang.org/x/text/language"
	"net"
	"net/http"
	"strconv"
	"strings"
)

func SetupWebApplication(router *mux.Router) *webapp.WebApplication {
	Log().Info("Initializing setup...")
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
	setupWebapp.GetAction("/", IndexAction)
	setupWebapp.GetAction("/step1", Step1Action)
	setupWebapp.PostAction("/step2", Step2Action)
	setupWebapp.PostAction("/step3", Step3Action)
	setupWebapp.PostAction("/install", InstallConfigAction)
	return setupWebapp
}

type ConfigBean struct {
	Values            *SystemConfiguration
	UseBridgedNetwork bool
	UseNATNetwork     bool
}

func readConfig(context *webapp.RequestContext) ConfigBean {
	context.Request.ParseMultipartForm(32000)
	bean := ConfigBean{
		Values: Container().Config,
	}
	config := Container().Config
	if context.GetParameter("ClusterLogin-ClusterState-Id") != "" {
		config.ClusterLogin.ClusterId =
			context.GetParameter("ClusterLogin-ClusterState-Id")
	}
	if context.GetParameter("ClusterLogin-ClusterState-Credentials") != "" {
		config.ClusterLogin.ClusterCredentials =
			context.GetParameter("ClusterLogin-ClusterState-Credentials")
	}
	// controller
	if context.GetParameter("IsController") != "" {
		config.ClusterConfig.LocallyManaged =
			util.ParseBool(context.GetParameter("IsController"))
		readClusterConfig(config.ClusterConfig, context)
		config.ClusterLogin.ClusterId = config.ClusterConfig.ClusterId
		config.ClusterLogin.ClusterCredentials = config.ClusterConfig.ClusterCredentials
		Log().Debug("In: ClusterConfig.LocallyManaged = " + strconv.FormatBool(config.ClusterConfig.LocallyManaged))
		Log().Debug("Applied: ClusterLogin based on current ClusterConfig")
	}
	readNetConfig(config, context)
	readNodeConfig(config, context)
	bean.UseBridgedNetwork = config.ClusterConfig.ClusterVMNet == Bridged
	bean.UseNATNetwork = config.ClusterConfig.ClusterVMNet == NAT
	return bean
}

func readNodeConfig(config *SystemConfiguration, context *webapp.RequestContext) {
	if context.GetParameter("IsMaster") != "" && config.MasterNode == nil {
		config.MasterNode = &LocalNodeConfig{
			NodeConfig: NodeConfig{
				NodeName:   "WinKube-" + config.ClusterLogin.ClusterId + "-Master",
				NodeType:   Master,
				NodeMemory: 2048,
				NodeCPU:    2,
			},
			NodeBox:        "ubuntu/xenial64",
			NodeBoxVersion: "20180831.0.0",
		}
	}
	readLocalNodeConfig(config.MasterNode, context, "master-")

	if context.GetParameter("IsWorker") != "" && config.WorkerNode == nil {
		config.WorkerNode = &LocalNodeConfig{
			NodeConfig: NodeConfig{
				NodeName:   "WinKube-" + config.ClusterLogin.ClusterId + "-Worker",
				NodeType:   Worker,
				NodeMemory: 2048,
				NodeCPU:    2,
			},
			NodeBox:        "ubuntu/xenial64",
			NodeBoxVersion: "20180831.0.0",
		}
	}
	readLocalNodeConfig(config.WorkerNode, context, "worker-")
}

func readNetConfig(config *SystemConfiguration, context *webapp.RequestContext) {
	if context.GetParameter("Net-MulticastEnabled") != "" {
		config.NetMulticastEnabled =
			util.ParseBool(context.GetParameter("Net-MulticastEnabled"))
		Log().Debug("In: Net-MulticastEnabled = " + strconv.FormatBool(config.NetMulticastEnabled))
	}
	if context.GetParameter("Net-UPnPPort") != "" {
		upnpPort, err := strconv.Atoi(context.GetParameter("Net-UPnPPort"))
		if err != nil {
			upnpPort = 1900
		}
		config.NetUPnPPort = upnpPort
		Log().Debug("In: Net-UPnpPort = " + strconv.Itoa(config.NetUPnPPort))
	}
	if context.GetParameter("Net-LookupMaster") != "" {
		config.NetLookupMaster =
			context.GetParameter("Net-LookupMaster")
		Log().Debug("In: Net-LookupMaster = " + config.NetLookupMaster)
	}
	if context.GetParameter("Net-Interface") != "" {
		config.NetHostInterface =
			context.GetParameter("Net-Interface")
		Log().Debug("In: Net-Interface = " + config.NetHostInterface)
	}
	if config.ClusterConfig.ClusterVMNet == UndefinedNetType {
		config.ClusterConfig.ClusterVMNet = NAT
		Log().Debug("Applied: ClusterConfig-VMNet = " + config.ClusterConfig.ClusterVMNet.String())
	}
}

func readClusterConfig(config *ClusterConfig, context *webapp.RequestContext) {
	if context.GetParameter("ClusterState-Id") != "" {
		config.ClusterId =
			context.GetParameter("ClusterState-Id")
	}
	if context.GetParameter("ClusterState-Credentials") != "" {
		config.ClusterCredentials =
			context.GetParameter("ClusterState-Credentials")
		Log().Debug("In: ClusterState-Credentials = " + config.ClusterCredentials)
	}
	if context.GetParameter("ClusterState-PodCIDR") != "" {
		config.ClusterPodCIDR =
			context.GetParameter("ClusterState-PodCIDR")
		Log().Debug("In: ClusterState-PodCIDR = " + config.ClusterPodCIDR)
	}
	if context.GetParameter("ClusterState-VMNet") != "" {
		val := context.GetParameter("ClusterState-VMNet")
		switch val {
		case "NAT":
		default:
			config.ClusterVMNet = NAT
		case "Bridged":
			config.ClusterVMNet = Bridged
		}
		Log().Debug("In: ClusterState-VMNet = " + config.ClusterVMNet.String())
	}
	if context.GetParameter("ClusterState-InternalNetCIDR") != "" {
		config.ClusterInternalNetCIDR =
			context.GetParameter("ClusterState-InternalNetCIDR")
		Log().Debug("In: ClusterState-InternalNetCIDR = " + config.ClusterInternalNetCIDR)
	}
	if context.GetParameter("ClusterState-MasterApiPort") != "" {
		port, err := strconv.Atoi(context.GetParameter("ClusterState-MasterApiPort"))
		if err == nil {
			config.ClusterMasterApiPort = port
			Log().Debug("In: ClusterState-MasterApiPort = " + strconv.Itoa(config.ClusterMasterApiPort))
		}
	}
	if context.GetParameter("ClusterState-NetCIDR") != "" {
		config.ClusterNetCIDR =
			context.GetParameter("ClusterState-NetCIDR")
		Log().Debug("In: ClusterState-NetCIDR = " + config.ClusterNetCIDR)
	}
	if context.GetParameter("ClusterState-ServiceDomain") != "" {
		config.ClusterServiceDomain =
			context.GetParameter("ClusterState-ServiceDomain")
		Log().Debug("In: ClusterState-ServiceDomain = " + config.ClusterServiceDomain)
	}
}

func readLocalNodeConfig(config *LocalNodeConfig, context *webapp.RequestContext, prefix string) {
	if context.GetParameter(prefix+"Node-NodeIP") != "" {
		config.NodeIP =
			context.GetParameter(prefix + "Node-NodeIP")
		Log().Debug("In: " + prefix + "NodeIP = " + config.NodeIP)
	}
	if context.GetParameter(prefix+"Node-Type") != "" {
		config.NodeType =
			nodeTypeFromString(context.GetParameter(prefix + "Node-Type"))
		Log().Debug("In: " + prefix + "Node-Type = " + config.NodeType.String())
	}
	if context.GetParameter(prefix+"Node-Name") != "" {
		config.NodeName =
			context.GetParameter(prefix + "Node-Name")
		Log().Debug("In: " + prefix + "Node-Name = " + config.NodeName)
	}
	if context.GetParameter(prefix+"Node-Box") != "" {
		config.NodeBox =
			context.GetParameter(prefix + "Node-Box")
		Log().Debug("In: " + prefix + "Node-Box = " + config.NodeBox)
	}
	if context.GetParameter(prefix+"Node-BoxVersion") != "" {
		config.NodeBoxVersion =
			context.GetParameter(prefix + "Node-BoxVersion")
		Log().Debug("In: " + prefix + "Node-BoxVersion = " + config.NodeBoxVersion)
	}
	if context.GetParameter(prefix+"Node-Memory") != "" {
		val, err := strconv.Atoi(context.GetParameter(prefix + "Node-Memory"))
		if err == nil {
			config.NodeMemory = val
			Log().Debug("In: " + prefix + "Node-Memory = " + strconv.Itoa(config.NodeMemory))
		}
	}
	if context.GetParameter(prefix+"Node-CPU") != "" {
		val, err := strconv.Atoi(context.GetParameter(prefix + "Node-CPU"))
		if err == nil {
			config.NodeCPU = val
			Log().Debug("In: " + prefix + "Node-CPU = " + strconv.Itoa(config.NodeCPU))
		}
	}
}

func nodeTypeFromString(nodeType string) NodeType {
	switch nodeType {
	case "Worker":
		return Worker
	case "Master":
		return Master
	case "Controller":
		return Controller
	default:
		return UndefinedNode
	}
}

func (this ConfigBean) WorkerNode() bool {
	return Container().Config.WorkerNode != nil
}
func (this ConfigBean) MasterNode() bool {
	return Container().Config.MasterNode != nil
}
func (this ConfigBean) ControllerNode() bool {
	return Container().Config.ClusterConfig != nil
}
func (this ConfigBean) UndefinedNode() bool {
	return !this.MasterNode() && !this.ControllerNode() && !this.WorkerNode()
}

// Web action starting the setup process

func IndexAction(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	if Container().CurrentStatus == APPSTATE_SETUP {
		data := make(map[string]interface{})
		data["Config"] = Container().Config
		return &webapp.ActionResponse{
			NextPage: "index",
			Model:    data,
		}
	}
	return &webapp.ActionResponse{
		NextPage: "_redirect",
		Model:    "/",
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
	clusterIds := (*clusterManager).GetKnownClusterIDs()
	// TODO remove this block
	clusterIds = append(clusterIds, "ClusterId-1")
	clusterIds = append(clusterIds, "ClusterId-2")
	clusterIds = append(clusterIds, "ClusterId-3")
	// TODO remove this block
	for _, id := range clusterIds {
		option := webapp.Option{
			Name:     id,
			Value:    id,
			Selected: Container().Config.ClusterLogin.ClusterId == id,
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
	// write config
	config := Container().Config
	_ = config.writeConfig()
	nodeManager := *Container().NodeManager
	_, err := nodeManager.ValidateConfig()
	if err != nil {
		fmt.Println("Validation failed: " + err.Error())
	} else {
		fmt.Println("Validation successful...")
	}
	clusterManager := *Container().ClusterManager
	if config.ClusterConfig.LocallyManaged {
		err = clusterManager.StartLocalController(config.ClusterConfig)
	} else {
		err = clusterManager.StartRemoteController(config.ClusterConfig)
	}
	if util.CheckAndLogError("Starting Cluster Manager", err) {
		action := nodeManager.Initialize(true)
		if util.CheckAndLogError("Starting Node Manager", action.Error) {
			nodeManager.StartNode()
			Container().RequiredAppStatus = APPSTATE_RUNNING
		}
	}
	return &webapp.ActionResponse{
		NextPage: "_redirect",
		Model:    "/actions",
	}
}
