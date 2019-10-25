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
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/winkube/service"
	"github.com/winkube/service/netutil"
	util2 "github.com/winkube/util"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func startup() {
	http.Handle("/", service.Container().Router)
	http.ListenAndServe("0.0.0.0:8080", nil)
	if !service.Container().Config.Ready() {
		explore("/setup")
	}
	go manageState()
}

func manageState() {
	for {
		if service.Container().RequiredAppStatus != service.Container().CurrentStatus {
			switch service.Container().RequiredAppStatus {
			case service.APPSTATE_SETUP:
				if service.Container().CurrentStatus == service.APPSTATE_RUNNING || service.Container().CurrentStatus == service.APPSTATE_ERROR {
					log.Info("Stopping service registry...")
					(*service.Container().ServiceRegistry).Stop()
				}
				log.Info("Entering setup mode...")
				service.Container().CurrentStatus = service.APPSTATE_SETUP
			case service.APPSTATE_RUNNING:
				if service.Container().Config.Ready() {
					service.Container().CurrentStatus = service.APPSTATE_STARTING
					log.Info("Starting WinKube...")
					if service.Container().Config.NetMulticastEnabled {
						log.Info("Starting UPnP multicast service registry...")
						(*service.Container().ServiceRegistry).StartUPnP(service.Container().ServiceProvider, service.Container().Config.NetUPnPPort)
					} else {
						log.Info("Starting catalogue service registry...")
						(*service.Container().ServiceRegistry).StartServiceCatalogue(service.Container().ServiceProvider, strings.Split(service.Container().Config.NetLookupMasters, ","))
					}
					// TODO Start node and register service in catalogue
					service.Container().CurrentStatus = service.APPSTATE_RUNNING
					log.Info("WinKube running.")
				}
			case service.APPSTATE_IDLE:
				if service.Container().CurrentStatus == service.APPSTATE_RUNNING {
					// TODO mark worker node as non deployable
					// wait for Kubernetes to remove workload
					service.Container().CurrentStatus = service.APPSTATE_IDLE
					log.Info("WinKube running.")
				}
			}
		}
		time.Sleep(10 * time.Second)
	}
}

// Opens the local browser with the setup application
func explore(path string) {
	// TODO also check for Linux/other browsers?
	cmd := exec.Command("explorer", path)
	err := cmd.Run()
	if err != nil {
		log.Panic("Cannopt open explorer...", err)
	}
}

func createDummyService(nodeConfig service.NodeConfig) netutil.Service {
	return netutil.Service{
		AdType:   service.WINKUBE_ADTYPE,
		Id:       nodeConfig.Id(),
		Service:  nodeConfig.NodeType.String(),
		Version:  "1",
		Location: nodeConfig.Host + ":" + strconv.Itoa(nodeConfig.Port),
		Server:   util2.RuntimeInfo() + " UPnP/1.0 WinKube/1.0",
		MaxAge:   60,
	}
}

// Handles the / (home URL)
func HomeHandler(w http.ResponseWriter, r *http.Request) {
	bytes, _ := json.Marshal(service.Container().Config.NodeConfig)
	w.Write(bytes)
}

// Handles the /cluster URL
func ClusterHandler(w http.ResponseWriter, r *http.Request) {
	bytes, _ := json.Marshal(service.Container().Config)
	w.Write(bytes)
}

// Main that starts the server and all services
func main() {
	fmt.Println("Starting management container...")
	service.Start()
	log.Info(service.Container().Stats())
	router := service.Container().Router
	if !service.Container().Config.Ready() {
		setupWebapp := service.SetupWebApplication(router)
		router.PathPrefix("/setup").HandlerFunc(setupWebapp.HandleRequest)
	}
	monitorWebapp := service.MonitorWebApplication(router)
	router.PathPrefix("/").HandlerFunc(monitorWebapp.HandleRequest)
	startup()
}
