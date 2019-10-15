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
 * Server command, whcih starts the communication server to establish the one-to-one communications between masters and
 * nodes.
 */
package main

import (
	"encoding/json"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"github.com/winkube/service/netutil"
	"github.com/winkube/service/runtime"
	"github.com/winkube/service/util"
	"github.com/winkube/webapp"
	"net/http"
	"os/exec"
	"strconv"
)

func setupWebApplication(router *mux.Router) *webapp.WebApplication {
	log.Info("Initializing setup...")
	setupWebapp := webapp.Create("WinKube-Setup", "/setup")
	// Pages
	setupWebapp.AddPage(&webapp.Page{
		Name:     "setup",
		Template: "templates/setup/index.html",
		Title:    "Welcome to WinKube - Setup",
	}).AddPage(&webapp.Page{
		Name:     "setup1",
		Template: "templates/setup/setup1.html",
		Title:    "WinKube Setup - Step 1",
	}).AddPage(&webapp.Page{
		Name:     "setup2",
		Template: "templates/setup/setup2.html",
		Title:    "WinKube Setup - Step 2",
	}).AddPage(&webapp.Page{
		Name:     "setup3",
		Template: "templates/setup/setup3.html",
		Title:    "WinKube Setup - Step 3",
	})
	// Actions
	setupWebapp.SetAction("/", &SetupAction{})
	setupWebapp.SetAction("/save-step1", &SaveSetup1Action{})
	setupWebapp.SetAction("/save-step2", &SaveSetup2Action{})
	setupWebapp.SetAction("/validate-config", &ValidateConfigAction{})
	return setupWebapp
}

func StartMuilticast() {
	log.Info("Starting UPnP multicast...")
	runtime.Container().ServiceRegistry.StartUPnP(&runtime.Container().ServiceProvider, 1900)
}

func startup() {
	http.Handle("/", runtime.Container().Router)
	http.ListenAndServe("0.0.0.0:8080", nil)
	if !runtime.Container().Config.Ready() {
		explore("/setup")
	}
}

// Web action starting the setup process
type SetupAction struct{}

func (a *SetupAction) DoAction(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	return &webapp.ActionResponse{
		NextPage: "setup",
		Model:    nil,
	}
}

// Web action continuing the setup process to step one
type SaveSetup1Action struct{}

func (a SaveSetup1Action) DoAction(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	return nil
}

// Web action continuing the setup process to step two
type SaveSetup2Action struct{}

func (a SaveSetup2Action) DoAction(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	return nil
}

// Web action continuing the setup process to validate the setup and start the node installation
type ValidateConfigAction struct{}

func (a ValidateConfigAction) DoAction(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	return nil
}

// Main that starts the server
func main() {
	log.Info("Starting management container...")
	log.Info(runtime.Container().Stats())
	if !runtime.Container().Config.Ready() {
		router := runtime.Container().Router
		setupWebapp := setupWebApplication(router)
		router.HandleFunc("/setup", setupWebapp.HandleRequest)
	}
	startup()
}

func explore(path string) {
	// TODO also check for Linux/other browsers?
	cmd := exec.Command("explorer", path)
	err := cmd.Run()
	if err != nil {
		log.Panic("Cannopt open explorer...", err)
	}
}

func createDummyService(nodeConfig runtime.NodeConfig) netutil.Service {
	return netutil.Service{
		AdType:   runtime.WINKUBE_ADTYPE,
		Id:       nodeConfig.Id(),
		Service:  nodeConfig.NodeType.String(),
		Version:  "1",
		Location: nodeConfig.Host + ":" + strconv.Itoa(nodeConfig.Port),
		Server:   util.RuntimeInfo() + " UPnP/1.0 WinKube/1.0",
		MaxAge:   60,
	}
}

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	bytes, _ := json.Marshal(runtime.Container().Config.NodeConfig)
	w.Write(bytes)
}

func ClusterHandler(w http.ResponseWriter, r *http.Request) {
	bytes, _ := json.Marshal(runtime.Container().Config)
	w.Write(bytes)
}
