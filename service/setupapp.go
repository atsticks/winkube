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
	setupWebapp.PostAction("/step1", Step1Action)
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
	config := Container().Config
	bean := ConfigBean{
		Values: config,
	}

	// controllerConnection
	if context.GetParameter("IsController") != "" {
		if util.ParseBool(context.GetParameter("IsController")) {
			config.InitControllerConfig()
			readClusterConfig(config.ControllerConfig, context)
			bean.UseBridgedNetwork = config.ControllerConfig.ClusterVMNet == Bridged
			bean.UseNATNetwork = config.ControllerConfig.ClusterVMNet == NAT
		}
	} else if context.Request.Method == "POST" &&
		context.GetParameter("selectNodes") != "" {
		config.ControllerConfig = nil
	}
	readClusterConnectionConfig(config, context)
	Log().Debug("In: ClusterConfig.LocallyManaged = " + strconv.FormatBool(config.ControllerConfig != nil))
	Log().Debug("Applied: ClusterControllerConnection based on current ClusterConfig")

	readNetConfig(config, context)
	readNodeConfig(config, context)
	if !config.IsControllerNode() {
		config.ControllerConfig = nil
	} else {
		config.ClusterLogin = nil
	}
	return bean
}

func readClusterConnectionConfig(config *SystemConfiguration, context *webapp.RequestContext) {
	if context.GetParameter("ClusterLogin-Cluster-Id") != "" {
		config.ClusterLogin.ClusterId =
			context.GetParameter("ClusterLogin-Cluster-Id")
	}
	if context.GetParameter("ClusterLogin-Credentials") != "" {
		config.ClusterLogin.ClusterCredentials =
			context.GetParameter("ClusterLogin-Credentials")
	}
	if context.GetParameter("ClusterLogin-Controller") != "" {
		config.ClusterLogin.ControllerHost =
			context.GetParameter("ClusterLogin-Controller")
	}
}

func readNodeConfig(config *SystemConfiguration, context *webapp.RequestContext) {
	if (context.GetParameter("IsPrimaryMaster") != "" || context.GetParameter("IsController") != "") && config.MasterNode == nil {
		config.MasterNode = &ClusterNodeConfig{
			IsJoiningNode:  false,
			NodeName:       "WinKube-" + config.ClusterId() + "-Master",
			NodeType:       Master,
			NodeMemory:     2048,
			NodeCPU:        2,
			NodeBox:        "ubuntu/xenial64",
			NodeBoxVersion: "20180831.0.0",
		}
	}
	if context.GetParameter("IsJoiningMaster") != "" &&
		(context.GetParameter("IsPrimaryMaster") == "" || context.GetParameter("IsController") == "") &&
		config.MasterNode == nil {
		config.MasterNode = &ClusterNodeConfig{
			IsJoiningNode:  true,
			NodeName:       "WinKube-" + config.ClusterId() + "-Master",
			NodeType:       Master,
			NodeMemory:     2048,
			NodeCPU:        2,
			NodeBox:        "ubuntu/xenial64",
			NodeBoxVersion: "20180831.0.0",
		}
		config.MasterNode.IsJoiningNode = true
	}
	if context.Request.Method == "POST" &&
		context.GetParameter("selectNodes") != "" &&
		context.GetParameter("IsJoiningMaster") == "" &&
		context.GetParameter("IsPrimaryMaster") == "" {
		config.MasterNode = nil
	}
	if config.MasterNode != nil {
		readLocalNodeConfig(config.MasterNode, context, "master-")
	}

	if context.GetParameter("IsWorker") != "" && config.WorkerNode == nil {
		config.WorkerNode = &ClusterNodeConfig{
			NodeName:       "WinKube-" + config.ClusterId() + "-Worker",
			NodeType:       Worker,
			NodeMemory:     2048,
			NodeCPU:        2,
			NodeBox:        "ubuntu/xenial64",
			NodeBoxVersion: "20180831.0.0",
		}
		config.WorkerNode.IsJoiningNode = true
	} else if context.Request.Method == "POST" &&
		context.GetParameter("selectNodes") != "" {
		config.WorkerNode = nil
	}
	if config.WorkerNode != nil {
		readLocalNodeConfig(config.WorkerNode, context, "worker-")
	}
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
	if context.GetParameter("Net-MasterController") != "" {
		config.MasterController =
			context.GetParameter("Net-MasterController")
		Log().Debug("In: Net-MasterController = " + config.MasterController)
	}
	if context.GetParameter("Net-Interface") != "" {
		config.NetHostInterface =
			context.GetParameter("Net-Interface")
		Log().Debug("In: Net-Interface = " + config.NetHostInterface)
	}
	if context.GetParameter("Net-Hostname") != "" {
		config.NetHostname =
			context.GetParameter("Net-Hostname")
		Log().Debug("In: Net-Hostname = " + config.NetHostname)
	}
	if config.ControllerConfig != nil && config.ControllerConfig.ClusterVMNet == UndefinedNetType {
		config.ControllerConfig.ClusterVMNet = NAT
		Log().Debug("Applied: ClusterConfig-VMNet = " + config.ControllerConfig.ClusterVMNet.String())
	}
}

func readClusterConfig(config *ClusterConfig, context *webapp.RequestContext) {
	if context.GetParameter("Cluster-Id") != "" {
		config.ClusterId =
			context.GetParameter("Cluster-Id")
	}
	if context.GetParameter("Cluster-Credentials") != "" {
		config.ClusterCredentials =
			context.GetParameter("Cluster-Credentials")
		Log().Debug("In: Cluster-Credentials = " + config.ClusterCredentials)
	}
	if context.GetParameter("Cluster-PodCIDR") != "" {
		config.ClusterPodCIDR =
			context.GetParameter("Cluster-PodCIDR")
		Log().Debug("In: Cluster-PodCIDR = " + config.ClusterPodCIDR)
	}
	if context.GetParameter("Cluster-VMNet") != "" {
		val := context.GetParameter("Cluster-VMNet")
		switch val {
		case "NAT":
		default:
			config.ClusterVMNet = NAT
		case "Bridged":
			config.ClusterVMNet = Bridged
		}
		Log().Debug("In: Cluster-VMNet = " + config.ClusterVMNet.String())
	}
	if context.GetParameter("Cluster-MasterApiPort") != "" {
		port, err := strconv.Atoi(context.GetParameter("Cluster-MasterApiPort"))
		if err == nil {
			config.ClusterMasterApiPort = port
			Log().Debug("In: Cluster-MasterApiPort = " + strconv.Itoa(config.ClusterMasterApiPort))
		}
	}
	if context.GetParameter("Cluster-NetCIDR") != "" {
		config.ClusterNetCIDR =
			context.GetParameter("Cluster-NetCIDR")
		Log().Debug("In: Cluster-NetCIDR = " + config.ClusterNetCIDR)
	}
	if context.GetParameter("Cluster-ServiceDomain") != "" {
		config.ClusterServiceDomain =
			context.GetParameter("Cluster-ServiceDomain")
		Log().Debug("In: Cluster-ServiceDomain = " + config.ClusterServiceDomain)
	}
}

func readLocalNodeConfig(config *ClusterNodeConfig, context *webapp.RequestContext, prefix string) {
	if context.GetParameter(prefix+"Node-NodeAddress") != "" {
		config.NodeAddress =
			context.GetParameter(prefix + "Node-NodeAddress")
		Log().Debug("In: " + prefix + "NodeAddress = " + config.NodeAddress)
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
	return Container().Config.ControllerConfig != nil
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
	data := make(map[string]interface{})
	bean := readConfig(context)
	if bean.Values.MasterNode == nil && !bean.Values.IsControllerNode() && !bean.Values.IsWorkerNode() {
		data["message"] = context.GetMessage("must-configure-anything.message")
		return &webapp.ActionResponse{
			NextPage: "step1",
			Model:    data,
		}
	}
	if bean.Values.IsJoiningMaster() && bean.Values.IsControllerNode() {
		data["message"] = context.GetMessage("first-master-must-primary.message")
		bean.Values.MasterNode.IsJoiningNode = false
	}
	if bean.Values.IsPrimaryMaster() && !bean.Values.IsControllerNode() {
		data["message"] = context.GetMessage("follow-master-must-joining.message")
		bean.Values.MasterNode.IsJoiningNode = true
	}
	if bean.Values.IsWorkerNode() && !bean.Values.IsMasterNode() && bean.Values.IsControllerNode() {
		data["message"] = context.GetMessage("master-created.message")
		bean.Values.InitMasterNode(true)
	}
	data["Config"] = bean
	data["Clusters"] = clusterOptions(Container().LocalController)
	data["Interfaces"] = interfaceOptions(Container().Config.NetHostInterface)
	// Check if node type is set...
	return &webapp.ActionResponse{
		NextPage: "step2",
		Model:    data,
	}
}

func clusterOptions(clusterManager *LocalController) webapp.Options {
	clusterOptions := webapp.Options{}
	// TODO fix this block
	clusterIds := []string{"ClusterId-1", "ClusterId-2", "ClusterId-3"} // (*clusterManager).GetKnownClusters()
	for _, id := range clusterIds {
		option := webapp.Option{
			Name:     id,
			Value:    id,
			Selected: Container().Config.ClusterId() == id,
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
	nodeManager := *Container().NodeManager
	action := (*GetActionManager()).StartAction("Validating configuration")
	defer action.Complete()
	err := nodeManager.ValidateConfig()
	if err != nil {
		defer action.CompleteWithError(err)
		data := make(map[string]interface{})
		bean := readConfig(context)
		data["Config"] = bean
		data["error"] = "Validation failed: " + err.Error()
		return &webapp.ActionResponse{
			NextPage: "step3",
			Model:    data,
		}
	} else {
		action.LogActionLn("Successfully validated.")
		Log().Info("Config validation successful.")
		_ = config.WriteConfig()
		action.LogActionLn("Resetting Nodes...")
		resetAction := (*Container().NodeManager).DestroyNodes()
		if action.OnErrorComplete(resetAction.Error) {
			Log().Error("Destroy Nodes failed: " + resetAction.Error.Error())
		} else {
			action.LogActionLn("Nodes destroyed.")
			Container().RequiredAppStatus = APPSTATE_RUNNING
		}
	}
	return &webapp.ActionResponse{
		NextPage: "_redirect",
		Model:    "/actions",
	}
}
