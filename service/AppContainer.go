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

package service

import (
	"fmt"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"github.com/winkube/service/netutil"
	util2 "github.com/winkube/util"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"golang.org/x/text/message/catalog"
	"gopkg.in/go-playground/validator.v9"
	"os"
	"sync"
	"time"
)

var container *AppContainer
var once sync.Once

const WINKUBE_ADTYPE = "winkube-service"
const WINKUBE_VERSION = "0.1"

type AppStatus int

const (
	APPSTATE_INITIALIZING AppStatus = iota
	APPSTATE_INITIALIZED
	APPSTATE_SETUP
	APPSTATE_STARTING
	APPSTATE_RUNNING
	APPSTATE_IDLE
	APPSTATE_ERROR
)

func Container() *AppContainer {
	if container == nil {
		Start()
	}
	return container
}

func Log() *log.Logger {
	return Container().Logger
}

func Start() {
	appContainer := AppContainer{
		Logger:            logger(),
		Router:            router(),
		Validator:         createValidator(),
		CurrentStatus:     APPSTATE_INITIALIZING,
		RequiredAppStatus: APPSTATE_RUNNING,
	}
	container = &appContainer
	appContainer.Config = config()
	appContainer.Router = router()
	appContainer.ServiceRegistry = netutil.InitServiceRegistry(WINKUBE_ADTYPE)
	appContainer.LocalController = CreateLocalController(container.ServiceRegistry)
	appContainer.CurrentStatus = APPSTATE_INITIALIZED
	appContainer.Logger.Info("WinKube is initialized, continue...")
}

func createValidator() *validator.Validate {
	val := validator.New()
	val.RegisterStructValidation(vagrantNodeValidation, VagrantNode{})
	val.RegisterStructValidation(vagrantConfig, VagrantConfig{})
	val.RegisterStructValidation(configValidation, SystemConfiguration{})
	return val
}

func vagrantConfig(sl validator.StructLevel) {
	config := sl.Current().Interface().(VagrantConfig)
	fmt.Printf("VagrantConfig: %+v", config)
}

func configValidation(sl validator.StructLevel) {
	config := sl.Current().Interface().(SystemConfiguration)
	if config.ControllerConfig == nil && config.ClusterLogin == nil {
		sl.ReportError(config, "ControllerConfig", "ClusterLogin", "Either a local cluster config (controllerConnection) or a cluster login must be provided", "")
	}
	if !config.NetConfig.NetMulticastEnabled && config.NetConfig.MasterController == "" {
		sl.ReportError(config, "MasterController", "ClusterConfig.NetMulticastEnabled", "Multicast is disabled, but no MasterController is configured", "")
	}
	if config.WorkerNode != nil && !config.WorkerNode.IsJoiningNode {
		sl.ReportError(config, "WorkerNode", "WorkerNode.IsJoiningNode", "A Worker must be joining always", "")
	}
	if config.ControllerConfig != nil {
		if config.ControllerConfig.ClusterToken == "" && config.MasterNode != nil && config.MasterNode.IsJoiningNode {
			sl.ReportError(config, "ClusterConfig.ClusterToken", "MasterNode.IsJoiningNode", "To join a cluster a ClusterToken is required.", "")
		}
		if config.ClusterLogin != nil {
			sl.ReportError(config, "ControllerConfig", "ClusterLogin", "Not both cluster login and local controllerConnection config can be active.", "")
		}
	}
}

func vagrantNodeValidation(sl validator.StructLevel) {
	config := sl.Current().Interface().(VagrantNode)
	if config.NodeType != Master && config.NodeType != Worker {
		sl.ReportError(config.NodeType, "NodeType", "IsMaster", "NodeType must be either Master or Worker", "")
	}
}

type AppContainer struct {
	Startup           time.Time
	StartupDuration   time.Duration
	Logger            *log.Logger
	MessageCatalog    *catalog.Builder
	Config            *SystemConfiguration
	ServiceProvider   *netutil.ServiceProvider
	Router            *mux.Router
	ServiceRegistry   *netutil.ServiceRegistry
	LocalController   *LocalController
	NodeManager       *NodeManager
	CurrentStatus     AppStatus
	RequiredAppStatus AppStatus
	Validator         *validator.Validate
}

func (this AppContainer) Stats() string {
	return "Container running (TODO startup and duration)"
}

func (this AppContainer) MessagePrinter(language language.Tag) *message.Printer {
	return message.NewPrinter(language, message.Catalog(this.MessageCatalog))
}

type DefaultServiceProvider struct {
	config *SystemConfiguration
}

func config() *SystemConfiguration {
	return InitConfig()
}
func logger() *log.Logger {
	fmt.Println("Initializing logging...")
	//log.SetFormatter(&log.JSONFormatter{}) // Log as JSON instead of the default ASCII formatter.
	log.SetFormatter(util2.NewPlainFormatter())

	// Output to stdout instead of the default stderr
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
	//log.SetReportCaller(true)
	log.WithFields(log.Fields{
		"app":    "kube-win",
		"node":   netutil.GetDefaultIP(),
		"server": util2.RuntimeInfo(),
	})
	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Working directory: " + dir)
	return log.StandardLogger()
}
func router() *mux.Router {
	log.Info("Initializing web application...")
	r := mux.NewRouter()
	return r
}
