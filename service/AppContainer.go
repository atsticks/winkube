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
	"os"
	"sync"
	"time"
)

var container *AppContainer
var once sync.Once

const WINKUBE_ADTYPE = "winkube-service"

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

func Start() {
	container = &AppContainer{
		Logger:            Logger(),
		Config:            Config(),
		Router:            Router(),
		NodeManager:       createNodeManager(),
		CurrentStatus:     APPSTATE_INITIALIZING,
		RequiredAppStatus: APPSTATE_RUNNING,
	}
	container.ActionManager = CreateActionManager()
	container.ServiceProvider = CreateServiceProvider(container.Config)
	container.ServiceRegistry = ServiceRegistry(container.ServiceProvider, WINKUBE_ADTYPE)
	container.ClusterManager = CreateClusterManager(container.ServiceRegistry)
	container.CurrentStatus = APPSTATE_INITIALIZED
	container.Logger.Info("WinKube is initialized, continue...")
}

type AppContainer struct {
	Startup           time.Time
	StartupDuration   time.Duration
	Logger            *log.Logger
	MessageCatalog    *catalog.Builder
	Config            *AppConfiguration
	ServiceProvider   *netutil.ServiceProvider
	Router            *mux.Router
	ServiceRegistry   *netutil.ServiceRegistry
	ClusterManager    *ClusterManager
	NodeManager       *NodeManager
	ActionManager     ActionManager
	CurrentStatus     AppStatus
	RequiredAppStatus AppStatus
}

func (this AppContainer) Stats() string {
	return "Container running (TODO startup and duration)"
}

func (this AppContainer) MessagePrinter(language language.Tag) *message.Printer {
	return message.NewPrinter(language, message.Catalog(this.MessageCatalog))
}

type DefaultServiceProvider struct {
	config *AppConfiguration
}

func (this DefaultServiceProvider) GetServices() []netutil.Service {
	// TODO implement on base of config and effective state of setup on this machine
	return []netutil.Service{}
}

// Dependeny Injection Module, provides logger and more...
func CreateServiceProvider(config *AppConfiguration) *netutil.ServiceProvider {
	log.Info("Initializing service provider...")
	var sp netutil.ServiceProvider = DefaultServiceProvider{
		config: config,
	}
	return &sp
}
func Config() *AppConfiguration {
	return CreateAppConfig("winkube.config", 1)
}
func Logger() *log.Logger {
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
func Router() *mux.Router {
	log.Info("Initializing web application...")
	r := mux.NewRouter()
	return r
}

func ServiceRegistry(serviceProvider *netutil.ServiceProvider, adType string) *netutil.ServiceRegistry {
	return netutil.InitServiceRegistry(adType, serviceProvider)
}
