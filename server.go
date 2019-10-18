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
	"github.com/winkube/service/runtime"
	"github.com/winkube/service/util"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func startup() {
	http.Handle("/", runtime.Container().Router)
	http.ListenAndServe("0.0.0.0:8080", nil)
	if !runtime.Container().Config.Ready() {
		explore("/setup")
	}
	go manageState()
}

func manageState() {
	for {
		if runtime.Container().RequiredAppStatus != runtime.Container().CurrentStatus {
			switch runtime.Container().RequiredAppStatus {
			case runtime.APPSTATE_SETUP:
				if runtime.Container().CurrentStatus == runtime.APPSTATE_RUNNING || runtime.Container().CurrentStatus == runtime.APPSTATE_ERROR {
					log.Info("Stopping service registry...")
					runtime.Container().ServiceRegistry.Stop()
				}
				log.Info("Entering setup mode...")
				runtime.Container().CurrentStatus = runtime.APPSTATE_SETUP
			case runtime.APPSTATE_RUNNING:
				if runtime.Container().Config.Ready() {
					runtime.Container().CurrentStatus = runtime.APPSTATE_STARTING
					log.Info("Starting WinKube...")
					if runtime.Container().Config.NetMulticastEnabled {
						log.Info("Starting UPnP multicast service registry...")
						runtime.Container().ServiceRegistry.StartUPnP(&runtime.Container().ServiceProvider, runtime.Container().Config.NetUPnPPort)
					} else {
						log.Info("Starting catalogue service registry...")
						runtime.Container().ServiceRegistry.StartServiceCatalogue(&runtime.Container().ServiceProvider, strings.Split(runtime.Container().Config.NetLookupMasters, ","))
					}
					// TODO Start node and register service in catalogue
					runtime.Container().CurrentStatus = runtime.APPSTATE_RUNNING
					log.Info("WinKube running.")
				}
			case runtime.APPSTATE_IDLE:
				if runtime.Container().CurrentStatus == runtime.APPSTATE_RUNNING {
					// TODO mark worker node as non deployable
					// wait for Kubernetes to remove workload
					runtime.Container().CurrentStatus = runtime.APPSTATE_IDLE
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

// Handles the / (home URL)
func HomeHandler(w http.ResponseWriter, r *http.Request) {
	bytes, _ := json.Marshal(runtime.Container().Config.NodeConfig)
	w.Write(bytes)
}

// Handles the /cluster URL
func ClusterHandler(w http.ResponseWriter, r *http.Request) {
	bytes, _ := json.Marshal(runtime.Container().Config)
	w.Write(bytes)
}

// Main that starts the server and all services
func main() {
	fmt.Println("Starting management container...")
	runtime.Start()
	log.Info(runtime.Container().Stats())
	if !runtime.Container().Config.Ready() {
		router := runtime.Container().Router
		setupWebapp := service.SetupWebApplication(router)
		router.PathPrefix("/setup").HandlerFunc(setupWebapp.HandleRequest)
	}
	startup()
}
