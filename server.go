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
	"github.com/winkube/service"
	"github.com/winkube/service/netutil"
	"github.com/winkube/service/util"
	"github.com/winkube/webapp"
	"net/http"
	"os"
	"os/exec"
	"strconv"
)

type RegistrationHandler struct {
	services []netutil.Service
}

func (RegistrationHandler) ServiceReceived(s netutil.Service) {
	service.GetCluster().RegisterService(s)
}

var configuration *AppConfiguration;
var registrationHandler RegistrationHandler
var multicast *netutil.Multicast
var setupWebapp *webapp.WebApplication

func init() {
	//log.SetFormatter(&log.JSONFormatter{}) // Log as JSON instead of the default ASCII formatter.
	log.SetFormatter(util.NewPlainFormatter())

	// Output to stdout instead of the default stderr
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
	//log.SetReportCaller(true)
	log.WithFields(log.Fields{
		"app":    "kube-win",
		"node":   netutil.GetInternalIP(),
		"server": netutil.RuntimeInfo(),
	}).Info("Win-Kube node starting...")
	// reading configuration
	configuration = ReadAppConfig("winkube-config.json")
}

func startSetup(r *mux.Router){
	setupWebapp = webapp.Create("WinKube-Setup")
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
	setupWebapp.SetAction("/", *SetupAction())
		.SetAction("/save-step1", *SaveSetup1Action())
		.SetAction("/save-step2", *SaveSetup2Action())
		.SetAction("/validate-config", *ValidateConfigAction())
	r.HandleFunc("/setup", setupWebapp.HandleRequest)
}

func startApplication(r *mux.Router) {
	if(configuration.MulticastEnabled){
		startMuilticast()
	}
	r.HandleFunc("/", HomeHandler)
	r.HandleFunc("/cluster", ClusterHandler)
}

func startMuilticast(){
	multicast = netutil.CreateMulticast(service.WINKUBE_ADTYPE, func() []netutil.Service {
		var model service.ServerInstance = service.GetInstance()
		return []netutil.Service{createService(model)}
	})
	log.Info("Multicast setup, registering handlers...")
	multicast.Listen(RegistrationHandler{
		services: []netutil.Service{},
	})
	log.Info("Starting Multicast...")
	multicast.StartAdvertizer()
}

func main() {
	log.Info("Starting web endpoints...")
	r := mux.NewRouter()
	if(!configuration.Ready()){
		startSetup(r)
		http.Handle("/", r)
		http.ListenAndServe("0.0.0.0:8080", nil)
		explore("/setup")
	}else {
		startApplication(r)
		http.Handle("/", r)
		// stay silent !
		http.ListenAndServe("0.0.0.0:8080", nil)
	}

}

func explore(path string) {
	// TODO also check for Linux/other browsers?
	cmd := exec.Command("explorer", path)
	err := cmd.Run()
	if err != nil {
		log.Panic("Cannopt open explorer...", err)
	}
}

func createService(model service.ServerInstance) netutil.Service {
	return netutil.Service{
		AdType:   service.WINKUBE_ADTYPE,
		Id:       model.Id(),
		Service:  model.InstanceRole,
		Version:  "1",
		Location: model.Host + ":" + strconv.Itoa(model.Port),
		Server:   netutil.RuntimeInfo(),
		MaxAge:   120,
	}
}

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	bytes, _ := json.Marshal(service.GetInstance())
	w.Write(bytes)
}

func ClusterHandler(w http.ResponseWriter, r *http.Request) {
	bytes, _ := json.Marshal(service.GetCluster())
	w.Write(bytes)
}


type SetupAction struct{}
func (a *SetupAction)doAction(req *webapp.RequestContext, writer http.ResponseWriter)*webapp.ActionResponse{
	return nil
}
type SaveSetup1Action struct{}
func (a *SaveSetup1Action)doAction(req *webapp.RequestContext, writer http.ResponseWriter)*webapp.ActionResponse{
	return nil
}
type SaveSetup2Action struct{}
func (a *SaveSetup2Action)doAction(req *webapp.RequestContext, writer http.ResponseWriter)*webapp.ActionResponse{
	return nil
}
type ValidateConfigAction struct{}
func (a *ValidateConfigAction)doAction(req *webapp.RequestContext, writer http.ResponseWriter)*webapp.ActionResponse{
	return nil
}
