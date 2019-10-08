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
	"github.com/gorilla/mux"
	"github.com/winkube/service"
	"github.com/winkube/service/netutil"
	"net/http"
	"strconv"
)

type RegistrationHandler struct {
	services []netutil.Service
}

func (RegistrationHandler) MsgReceived(s netutil.Service) {
	service.GetCluster().RegisterService(s)
}

var registrationHandler RegistrationHandler

func main() {
	mc := netutil.GetMulticast()
	registrationHandler = RegistrationHandler{
		services: []netutil.Service{},
		//Age: 240,
	}

	go mc.StartAdvertizer(func() netutil.Service {
		var model service.InstanceModel = service.GetInstanceModel()
		return createService(model)
	})
	fmt.Println("Starting rest endpoint...")
	r := mux.NewRouter()
	r.HandleFunc("/", HomeHandler)
	r.HandleFunc("/cluster", ClusterHandler)
	r.HandleFunc("/setup", SetupHandler)
	http.Handle("/", r)

	http.ListenAndServe("0.0.0.0:8080", nil)
}

func createService(model service.InstanceModel) netutil.Service {
	return netutil.Service{
		AdType:   "com.gh.atsticks.winkube",
		Usn:      model.Id(),
		Service:  model.InstanceRole,
		Location: model.Host + ":" + strconv.Itoa(model.Port),
		MaxAge:   120,
	}
}

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	bytes, _ := json.Marshal(service.GetInstanceModel())
	w.Write(bytes)
}

func ClusterHandler(w http.ResponseWriter, r *http.Request) {
	bytes, _ := json.Marshal(service.GetCluster())
	w.Write(bytes)
}

func SetupHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Setup..."))
}
