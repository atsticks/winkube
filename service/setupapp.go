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
	setupWebapp.SetAction("/save-step1", &SaveStep1Action{})
	setupWebapp.SetAction("/save-step2", &SaveStep2Action{})
	setupWebapp.SetAction("/validate-config", &ValidateConfigAction{})
	return setupWebapp
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
	data["Config"] = *runtime.Container().Config
	return &webapp.ActionResponse{
		NextPage: "step1",
		Model:    data,
	}
}

// Web action continuing the setup process to step one
type SaveStep1Action struct{}

func (a SaveStep1Action) DoAction(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	joinCluster := context.GetQueryParameter("setup-mode") == "join_cluster"
	runtime.Container().Config.UseExistingCluster = joinCluster
	data := make(map[string]interface{})
	data["Config"] = *runtime.Container().Config

	ifaceOptions := interfaceOptions(runtime.Container().Config.NetHostInterface)
	data["interfaces"] = ifaceOptions

	runtime.Container().Config.NetMulticastEnabled, _ = strconv.ParseBool(context.GetParameter("NetMulticastEnabled"))
	runtime.Container().Config.NetUPnPPort, _ = strconv.Atoi(context.GetParameter("NetUPnPPort"))
	runtime.Container().Config.NetLookupMasters = context.GetParameter("NetLookupMasters")
	runtime.Container().Config.ClusterID = context.GetParameter("ClusterID")
	runtime.Container().Config.ClusterCredentials = context.GetParameter("ClusterCredentials")
	runtime.Container().Config.ClusterPodCIDR = context.GetParameter("ClusterPodCIDR")
	runtime.Container().Config.ClusterVMNet = context.GetParameter("ClusterVMNet")
	runtime.Container().Config.NodeNetBridgeCIDR = context.GetParameter("NodeNetBridgeCIDR")
	runtime.Container().Config.NodeNetNodeIP = context.GetParameter("NodeNetNodeIP")
	switch context.GetParameter("NodeType") {
	case "WorkerNode":
		runtime.Container().Config.NodeType = runtime.WorkerNode
	case "MasterNode":
		runtime.Container().Config.NodeType = runtime.MasterNode
	case "MonitorNode":
		runtime.Container().Config.NodeType = runtime.MonitorNode
	default:
		runtime.Container().Config.NodeType = runtime.Undefined
	}

	return &webapp.ActionResponse{
		NextPage: "step2",
		Model:    data,
	}
}

func interfaceOptions(selected string) webapp.Options {
	interfacesOption := webapp.Options{}
	ifaces, _ := net.Interfaces() //here your interface
	for _, iface := range ifaces {
		option := webapp.Option{
			Name:     iface.Name + "( " + iface.HardwareAddr.String() + ")" + getDisplayAddr(iface.Addrs()),
			Value:    iface.Name,
			Selected: runtime.Container().Config.NetHostInterface == iface.Name,
		}
		interfacesOption.Entries = append(interfacesOption.Entries, option)
	}
	return interfacesOption
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
type SaveStep2Action struct{}

func (a SaveStep2Action) DoAction(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	// Collect messages
	data := make(map[string]interface{})
	data["Config"] = *runtime.Container().Config
	return &webapp.ActionResponse{
		NextPage: "step3",
		Model:    data,
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
