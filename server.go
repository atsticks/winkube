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
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/winkube/service"
	"github.com/winkube/service/assert"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

func startup() {
	if !service.Container().Config.Ready() {
		service.Container().RequiredAppStatus = service.APPSTATE_SETUP
	}
	go manageState()
	http.Handle("/", service.Container().Router)
	http.ListenAndServe("0.0.0.0:8080", nil)
}

func manageState() {
	for {

		if service.Container().RequiredAppStatus != service.Container().CurrentStatus {
			var actionManager service.ActionManager = *service.GetActionManager()
			config := service.Container().Config
			var action *service.Action
			switch service.Container().RequiredAppStatus {
			case service.APPSTATE_SETUP:
				action = actionManager.StartAction("Trying to switch to SETUP Mode")
				if service.Container().CurrentStatus == service.APPSTATE_RUNNING || service.Container().CurrentStatus == service.APPSTATE_ERROR {
					log.Info("Stopping service registry...")
					action.LogActionLn("Stopping service registry...")
					(*service.Container().ServiceRegistry).Stop()
					action.LogActionLn("Service registry stopped.")
				}
				log.Info("Entering setup mode...")
				service.Container().CurrentStatus = service.APPSTATE_SETUP
				action.CompleteWithMessage("New Mode applied: SETUP")
			case service.APPSTATE_RUNNING:
				switchToRunning(config)
			case service.APPSTATE_IDLE:
				if service.Container().CurrentStatus == service.APPSTATE_RUNNING {
					action = actionManager.StartAction("Switch to IDLE Mode")
					// TODO mark worker node as non deployable
					// wait for Kubernetes to remove workload
					log.Info("WinKube now idle.")
					service.Container().CurrentStatus = service.APPSTATE_IDLE
					action.CompleteWithMessage("New Mode applied: IDLE")
				}
			}
		}
		time.Sleep(10 * time.Second)
	}
}

func switchToRunning(config *service.SystemConfiguration) {
	actionManager := *service.GetActionManager()
	action := actionManager.StartAction("Trying to switch to RUNNING Mode")
	defer action.Complete()
	oldStatus := service.Container().CurrentStatus
	if config.Ready() {
		service.Container().CurrentStatus = service.APPSTATE_STARTING
		action.LogActionLn("Starting service registry...")
		(*service.Container().ServiceRegistry).Start(config.NetConfig.NetMulticastEnabled, config.NetConfig.NetUPnPPort, strings.Split(config.NetConfig.MasterController, ","))
		action.LogActionLn("Starting cluster controller...")
		err := (*service.Container().LocalController).Start(config)
		if action.OnErrorComplete(err) {
			log.Error("Starting local controller failed: " + err.Error())
			resetToSetupStatus(oldStatus)
			return
		}
		action.LogActionLn("Resetting nodes...")
		resetAction := (*service.Container().NodeManager).DestroyNodes()
		if action.OnErrorComplete(resetAction.Error) {
			log.Error("Destrey nodes failed: " + resetAction.Error.Error())
			resetToSetupStatus(oldStatus)
			return
		}
		action.LogActionLn("Configuring nodes...")
		clusterConfig := (*service.Container().LocalController).GetClusterConfig()
		assert.AssertNotNil(clusterConfig)
		configAction := (*service.Container().NodeManager).ConfigureNodes(*config, clusterConfig, true)
		if !action.OnErrorComplete(configAction.Error) {
			log.Error("Failed to configure the nodes: ", configAction.Error)
			log.Info("Starting nodes...")
			action.LogActionLn("Starting nodes...")
			startNodesAction := (*service.Container().NodeManager).StartNodes()
			if action.OnErrorComplete(startNodesAction.Error) {
				log.Error("Starting nodes failed: " + startNodesAction.Error.Error())
				resetToSetupStatus(oldStatus)
				return
			}
			log.Info("Registering services...")
			service.Container().CurrentStatus = service.APPSTATE_RUNNING
		} else {
			log.Error("Configure nodes failed: " + configAction.Error.Error())
			resetToSetupStatus(oldStatus)
			action.CompleteWithMessage("Cannot switch to a RUNNING state: config is not ready.")
		}
	} else {
		action.CompleteWithMessage("Cannot switch to a RUNNING state: config is not ready.")
		resetToSetupStatus(oldStatus)
	}
}

func resetToSetupStatus(oldStatus service.AppStatus) {
	service.Container().CurrentStatus = oldStatus
	service.Container().RequiredAppStatus = service.APPSTATE_SETUP
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

// Main that starts the server and all services
func main() {
	fmt.Println("Starting management container...")
	service.Start()
	log.Info(service.Container().Stats())
	router := service.Container().Router
	setupWebapp := service.SetupWebApplication(router)
	router.PathPrefix("/setup").HandlerFunc(setupWebapp.HandleRequest)
	monitorWebapp := service.MonitorWebApplication(router)
	router.PathPrefix("/").HandlerFunc(monitorWebapp.HandleRequest)
	startup()
}
