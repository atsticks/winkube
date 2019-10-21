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
	log "github.com/sirupsen/logrus"
	"github.com/winkube/service/runtime"
	"github.com/winkube/webapp"
	"golang.org/x/text/language"
	"net"
	"net/http"
	"strconv"
	"strings"
)

func SetupWebApplication(router *mux.Router) *webapp.WebApplication {
	log.Info("Initializing setup...")
	setupWebapp := webapp.Create("WinKube-Setup", "/setup", language.English)
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
	setupWebapp.SetAction("/", &IndexAction{})
	setupWebapp.SetAction("/step1", &Step1Action{})
	setupWebapp.SetAction("/step2", &Step2Action{})
	setupWebapp.SetAction("/step3", &Step3Action{})
	setupWebapp.SetAction("/validate-config", &ValidateConfigAction{})
	return setupWebapp
}

type ConfigBean struct {
	Values *runtime.AppConfiguration
}

func readConfig(context *webapp.RequestContext) ConfigBean {
	context.Request.ParseMultipartForm(32000)
	bean := ConfigBean{
		Values: runtime.Container().Config,
	}
	if context.GetParameter("UseExistingCluster") != "" {
		runtime.Container().Config.UseExistingCluster =
			ParseBool(context.GetParameter("UseExistingCluster"))
	}
	if context.GetParameter("Net-MulticastEnabled") != "" {
		runtime.Container().Config.NetMulticastEnabled =
			ParseBool(context.GetParameter("Net-MulticastEnabled"))
	}
	if context.GetParameter("Net-UPnPPort") != "" {
		upnpPort, err := strconv.Atoi(context.GetParameter("Net-UPnPPort"))
		if err != nil {
			upnpPort = 1900
		}
		runtime.Container().Config.NetUPnPPort = upnpPort
	}
	if context.GetParameter("Net-LookupMasters") != "" {
		runtime.Container().Config.NetLookupMasters =
			context.GetParameter("Net-LookupMasters")
	}
	if context.GetParameter("Net-Interface") != "" {
		runtime.Container().Config.NetHostInterface =
			context.GetParameter("Net-Interface")
	}
	if context.GetParameter("Cluster-ID") != "" {
		runtime.Container().Config.ClusterID =
			context.GetParameter("Cluster-ID")
	}
	if context.GetParameter("Cluster-Credentials") != "" {
		runtime.Container().Config.ClusterCredentials =
			context.GetParameter("Cluster-Credentials")
	}
	if context.GetParameter("Cluster-PodCIDR") != "" {
		runtime.Container().Config.ClusterPodCIDR =
			context.GetParameter("Cluster-PodCIDR")
	}
	if context.GetParameter("Cluster-NetType") != "" {
		runtime.Container().Config.ClusterVMNet =
			context.GetParameter("Cluster-NetType")
	}
	if context.GetParameter("Node-NetBridgeCIDR") != "" {
		runtime.Container().Config.NodeNetBridgeCIDR =
			context.GetParameter("Node-NetBridgeCIDR")
	}
	if context.GetParameter("Node-NetNodeIP") != "" {
		runtime.Container().Config.NodeNetNodeIP =
			context.GetParameter("Node-NetNodeIP")
	}
	if context.GetParameter("Node-Type") != "" {
		runtime.Container().Config.NodeType =
			nodeTypeFromString(context.GetParameter("Node-Type"))
	}
	return bean
}
func nodeTypeFromString(nodeType string) runtime.NodeType {
	switch nodeType {
	case "WorkerNode":
		return runtime.WorkerNode
	case "MasterNode":
		return runtime.MasterNode
	case "MonitorNode":
		return runtime.MonitorNode
	default:
		return runtime.Undefined
	}
}

func (this ConfigBean) WorkerNode() bool {
	return runtime.Container().Config.NodeType == runtime.WorkerNode
}
func (this ConfigBean) MasterNode() bool {
	return runtime.Container().Config.NodeType == runtime.MasterNode
}
func (this ConfigBean) MonitorNode() bool {
	return runtime.Container().Config.NodeType == runtime.MonitorNode
}
func (this ConfigBean) UndefinedNode() bool {
	return runtime.Container().Config.NodeType == runtime.Undefined
}
func (this ConfigBean) UseBridgedNetwork() bool {
	return runtime.Container().Config.ClusterVMNet == "Bridged"
}
func (this ConfigBean) UseNATNetwork() bool {
	return runtime.Container().Config.ClusterVMNet == "NAT"
}

// Web action starting the setup process
type IndexAction struct{}

func (a *IndexAction) DoAction(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	data := make(map[string]interface{})
	data["Config"] = *runtime.Container().Config
	return &webapp.ActionResponse{
		NextPage: "index",
		Model:    data,
	}
}

type Step1Action struct{}

func (a *Step1Action) DoAction(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	// Collect messages
	data := make(map[string]interface{})
	bean := readConfig(context)
	data["Config"] = bean
	return &webapp.ActionResponse{
		NextPage: "step1",
		Model:    data,
	}
}

// Web action continuing the setup process to step one
type Step2Action struct{}

func (a Step2Action) DoAction(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	bean := readConfig(context)
	data := make(map[string]interface{})
	data["Config"] = bean
	data["Clusters"] = clusterOptions(runtime.Container().ClusterManager)
	data["Interfaces"] = interfaceOptions(runtime.Container().Config.NetHostInterface)

	return &webapp.ActionResponse{
		NextPage: "step2",
		Model:    data,
	}
}

func clusterOptions(clusterManager runtime.ClusterManager) webapp.Options {
	clusterOptions := webapp.Options{}
	clusterIds := clusterManager.GetClusterIDs()
	// TODO remove this block
	clusterIds = append(clusterIds, "ClusterID-1")
	clusterIds = append(clusterIds, "ClusterID-2")
	clusterIds = append(clusterIds, "ClusterID-3")
	// TODO remove this block
	for _, id := range clusterIds {
		option := webapp.Option{
			Name:     id,
			Value:    id,
			Selected: runtime.Container().Config.ClusterID == id,
		}
		clusterOptions.Entries = append(clusterOptions.Entries, option)
	}
	return clusterOptions
}

func interfaceOptions(selected string) webapp.Options {
	ifCurrent := runtime.Container().Config.NetHostInterface
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
type Step3Action struct{}

func (a Step3Action) DoAction(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	// Collect messages
	data := make(map[string]interface{})
	bean := readConfig(context)
	data["Config"] = bean
	return &webapp.ActionResponse{
		NextPage: "step3",
		Model:    data,
	}
}

// ParseBool returns the boolean value represented by the string.
// It accepts 1, t, T, TRUE, true, True, 0, f, F, FALSE, false, False.
// Any other value returns an error.
func ParseBool(str string) bool {
	switch str {
	case "1", "t", "T", "true", "TRUE", "True", "on", "On", "ON":
		return true
	case "0", "f", "F", "false", "FALSE", "False", "off", "Off", "OFF":
		return false
	default:
		return false
	}
}

// Web action starting the node after the configuration has been completed
type ValidateConfigAction struct{}

func (a ValidateConfigAction) DoAction(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	data := make(map[string]interface{})
	data["Config"] = *runtime.Container().Config
	return &webapp.ActionResponse{
		NextPage: "startNode",
		Model:    data,
	}
}
