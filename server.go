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
	"net/http"
	"os"
	"strconv"
)

type RegistrationHandler struct {
	services []netutil.Service
}

func (RegistrationHandler) ServiceReceived(s netutil.Service) {
	service.GetCluster().RegisterService(s)
}

var registrationHandler RegistrationHandler
var multicast netutil.Multicast

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
}

func main() {
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

	log.Info("Starting rest endpoints...")
	r := mux.NewRouter()
	r.HandleFunc("/", HomeHandler)
	r.HandleFunc("/cluster", ClusterHandler)
	r.HandleFunc("/setup", SetupHandler)
	http.Handle("/", r)

	http.ListenAndServe("0.0.0.0:8080", nil)
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

func SetupHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Setup..."))
}
